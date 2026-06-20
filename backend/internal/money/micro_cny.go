package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const Scale int64 = 1_000_000

const maxScaledCNY = int64(^uint64(0)>>1) / Scale

type MicroCNY int64

func FromCNYString(input string) (MicroCNY, error) {
	if input == "" {
		return 0, errors.New("money string is empty")
	}
	if strings.HasPrefix(input, "-") {
		return 0, errors.New("money cannot be negative")
	}

	parts := strings.Split(input, ".")
	if len(parts) > 2 {
		return 0, errors.New("invalid money format")
	}

	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid yuan amount: %w", err)
	}

	var fractional int64
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) > 6 {
			return 0, errors.New("money supports at most 6 decimal places")
		}
		frac = frac + strings.Repeat("0", 6-len(frac))
		fractional, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid fractional amount: %w", err)
		}
	}

	if yuan > maxScaledCNY || (yuan == maxScaledCNY && fractional > int64(^uint64(0)>>1)%Scale) {
		return 0, errors.New("money amount overflows micro cny range")
	}

	return MicroCNY(yuan*Scale + fractional), nil
}

func (m MicroCNY) FormatCNY() string {
	amount := int64(m)
	if amount < 0 {
		yuan := -(amount / Scale)
		fractional := -(amount % Scale)
		return fmt.Sprintf("-%d.%06d", yuan, fractional)
	}

	yuan := amount / Scale
	fractional := amount % Scale
	return fmt.Sprintf("%d.%06d", yuan, fractional)
}
