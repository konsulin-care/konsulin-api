package constvars

const (
	RegexContainAtLeastOneSpecialChar = `.*[!@#$%^&*(),.?":{}|<>].*`
	RegexContainAtLeastOneUppercase   = `.*[A-Z].*`
	RegexContainAtLeastOneLowercase   = `.*[a-z].*`
	RegexContainAtLeastOneDigit       = `.*\d.*`
	RegexEmail                        = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	RegexAlphanumeric                 = `^[a-zA-Z0-9]+$`
	RegexAlphabetic                   = `^[a-zA-Z]+$`
	RegexNumeric                      = `^\d+$`
	RegexURL                          = `^(http|https):\/\/[^\s$.?#].[^\s]*$`
	RegexIPv4                         = `^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`
	RegexIPv6                         = `^([0-9a-fA-F]{1,4}:){7}([0-9a-fA-F]{1,4}|:)$`
	RegexDateYYYYMMDD                 = `^\d{4}-\d{2}-\d{2}$`
	RegexTimeHHMMSS                   = `^\d{2}:\d{2}:\d{2}$`
	RegexDateTimeISO8601              = `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$`
	RegexHexColorCode                 = `^#?([a-fA-F0-9]{6}|[a-fA-F0-9]{3})$`
	RegexIndonesiaPhoneNumber         = `^(?:\+62|62|0)8[1-9][0-9]{6,10}$`
	RegexIndonesiaZIPCode             = `^\d{5}$`
	RegexPhoneNumberGeneral           = `^\+[1-9]\d{9,14}$`
	// RegexPhoneNumberDigitsInternational matches "E.164 without plus", digits only.
	// 10-15 digits, cannot start with 0.
	RegexPhoneNumberDigitsInternational = `^[1-9]\d{9,14}$`
)
