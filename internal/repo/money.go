package repo

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrUnsupportedCurrency indicates the currency is outside V1 allowed set.
var ErrUnsupportedCurrency = errors.New("unsupported currency")

// NormalizeCurrencyCode validates and normalizes V1 currencies.
func NormalizeCurrencyCode(s string) (string, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	switch s {
	case "USD", "JPY":
		return s, nil
	default:
		return "", ErrUnsupportedCurrency
	}
}

// ParseMajorToMinor converts a display amount to minor units with currency precision rules.
func ParseMajorToMinor(currency, major string) (int64, error) {
	currency, err := NormalizeCurrencyCode(currency)
	if err != nil {
		return 0, err
	}
	major = strings.TrimSpace(major)
	if major == "" {
		return 0, errors.New("amount is required")
	}
	negative := strings.HasPrefix(major, "-")
	if negative {
		major = strings.TrimPrefix(major, "-")
	}
	parts := strings.SplitN(major, ".", 2)
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, errors.New("invalid amount format")
	}
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}

	var minor int64
	switch currency {
	case "USD":
		if len(frac) > 2 {
			return 0, errors.New("USD supports up to 2 decimal places")
		}
		if len(frac) == 1 {
			frac += "0"
		} else if len(frac) == 0 {
			frac = "00"
		}
		f, err := strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, errors.New("invalid amount format")
		}
		minor = whole*100 + f
	case "JPY":
		if frac != "" {
			return 0, errors.New("JPY does not allow decimal places")
		}
		minor = whole
	default:
		return 0, ErrUnsupportedCurrency
	}
	if negative {
		minor = -minor
	}
	return minor, nil
}

// FormatMinorForDisplay converts minor units to display string by currency rules.
func FormatMinorForDisplay(currency string, amountMinor int64) (string, error) {
	currency, err := NormalizeCurrencyCode(currency)
	if err != nil {
		return "", err
	}
	if currency == "JPY" {
		return strconv.FormatInt(amountMinor, 10), nil
	}
	sign := ""
	if amountMinor < 0 {
		sign = "-"
		amountMinor = -amountMinor
	}
	whole := amountMinor / 100
	frac := amountMinor % 100
	return fmt.Sprintf("%s%d.%02d", sign, whole, frac), nil
}
