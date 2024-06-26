package tax

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

func TaxCalculationsHandler(c echo.Context) error {
	if c.Request().Body == http.NoBody {
		return c.JSON(http.StatusBadRequest, "Empty request body")
	}

	itd := IncomeTaxDetail{}
	err := c.Bind(&itd)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := validateTaxValues(&itd); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	tcr := taxCalculate(itd)

	return c.JSON(http.StatusOK, tcr)
}

func TaxUploadCsvHandler(c echo.Context) error {
	if c.Request().Body == http.NoBody {
		return c.JSON(http.StatusBadRequest, "Empty request body")
	}

	file, err := c.FormFile("taxFile")
	if err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, err)
	}
	defer src.Close()

	reader := csv.NewReader(src)

	records, err := reader.ReadAll()
	if err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	result := TaxCsvResult{}
	for i, record := range records {
		if i == 0 {
			continue // Skip header row
		}

		if err := validateCsvData(record); err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}

		totalIncome, _ := strconv.ParseFloat(record[0], 64)
		wht, _ := strconv.ParseFloat(record[1], 64)
		donation, _ := strconv.ParseFloat(record[2], 64)

		itd := IncomeTaxDetail{
			TotalIncome:    totalIncome,
			WithHoldingTax: wht,
			Allowances: []Allowance{
				{AllowanceType: "donation", Amount: donation},
			},
		}

		if err := validateTaxValues(&itd); err != nil {
			fmt.Println(err)
			return c.JSON(http.StatusBadRequest, err)
		}

		taxCal := taxCalculate(itd)

		var resultDetail TaxCsvResultDetail
		if taxCal.TotalTax >= 0 {
			resultDetail = TaxCsvResultDetail{
				TotalIncome: totalIncome,
				Tax:         taxCal.TotalTax,
			}
		} else {
			resultDetail = TaxCsvResultDetail{
				TotalIncome: totalIncome,
				TaxRefund:   taxCal.TotalTax * -1.0,
			}
		}
		result.Taxes = append(result.Taxes, resultDetail)
		//fmt.Printf("totalIncome: %.2f, wht: %.2f, donation: %.2f\n", totalIncome, wht, donation)
	}

	return c.JSON(http.StatusOK, result)
}

func validateCsvData(record []string) error {
	for _, field := range record {
		if strings.TrimSpace(field) == "" {
			return fmt.Errorf("all values must be non-empty")
		}
	}

	for _, field := range record {
		if _, err := strconv.ParseFloat(field, 64); err != nil {
			return fmt.Errorf("invalid format")
		}
	}

	return nil
}

func validateTaxValues(t *IncomeTaxDetail) error {
	if t.TotalIncome < 0 {
		return fmt.Errorf("total income (%.2f) cannot be negative", t.TotalIncome)
	}

	if t.WithHoldingTax < 0 {
		return fmt.Errorf("wht (%.2f) cannot be negative", t.WithHoldingTax)
	}

	if t.WithHoldingTax > t.TotalIncome {
		return fmt.Errorf("wht (%.2f) cannot be greater than total income (%.2f)", t.WithHoldingTax, t.TotalIncome)
	}

	for _, allowance := range t.Allowances {
		if strings.Contains(strings.ToLower(allowance.AllowanceType), "personal") || strings.Contains(strings.ToLower(allowance.AllowanceType), "receipt") || allowance.AllowanceType == "donation" {
			if allowance.Amount < 0 {
				return fmt.Errorf("allowance amount (%.2f) cannot be negative", allowance.Amount)
			}
		} else {
			return fmt.Errorf("invalid allowance type: %s", allowance.AllowanceType)
		}
	}

	return nil
}
