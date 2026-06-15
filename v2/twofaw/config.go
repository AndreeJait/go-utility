package twofaw

const (
	digitsDefault = 6
	periodDefault = 30
	issuerDefault = "go-utility"
)

func (c *Config) issuer() string {
	if c == nil || c.Issuer == "" {
		return issuerDefault
	}
	return c.Issuer
}

func (c *Config) digits() int {
	if c == nil || c.Digits == 0 {
		return digitsDefault
	}
	return c.Digits
}

func (c *Config) period() uint {
	if c == nil || c.Period == 0 {
		return periodDefault
	}
	return c.Period
}
