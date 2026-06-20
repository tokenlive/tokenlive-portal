package money

import "testing"

func TestFromCNYString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    MicroCNY
		wantErr bool
	}{
		{name: "integer yuan", input: "1", want: 1_000_000},
		{name: "six decimals", input: "0.123456", want: 123_456},
		{name: "pads decimals", input: "2.5", want: 2_500_000},
		{name: "largest safe input", input: "9223372036854.775807", want: MicroCNY(9223372036854775807)},
		{name: "largest safe integer yuan", input: "9223372036854", want: MicroCNY(9223372036854000000)},
		{name: "first overflowing fractional input", input: "9223372036854.775808", wantErr: true},
		{name: "first overflowing integer input", input: "9223372036855", wantErr: true},
		{name: "rejects too many decimals", input: "1.1234567", wantErr: true},
		{name: "rejects negative", input: "-1", wantErr: true},
		{name: "rejects empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromCNYString(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFormatCNY(t *testing.T) {
	tests := []struct {
		input MicroCNY
		want  string
	}{
		{input: 1_000_000, want: "1.000000"},
		{input: 123_456, want: "0.123456"},
		{input: 2_500_000, want: "2.500000"},
		{input: -1, want: "-0.000001"},
		{input: -1_000_001, want: "-1.000001"},
		{input: MicroCNY(-9223372036854775808), want: "-9223372036854.775808"},
	}

	for _, tt := range tests {
		if got := tt.input.FormatCNY(); got != tt.want {
			t.Fatalf("got %s, want %s", got, tt.want)
		}
	}
}
