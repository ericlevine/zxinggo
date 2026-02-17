// Package charset provides character set ECI mappings and encoding detection.
package charset

import "errors"

// ErrFormatECI indicates an invalid ECI value.
var ErrFormatECI = errors.New("charset: invalid ECI value")

// ECI represents a Character Set Extended Channel Interpretation.
type ECI struct {
	Value    int
	Name     string
	GoName   string // Go encoding name
	Aliases  []string
}

// pre-defined ECIs
var (
	ECICp437      = &ECI{0, "Cp437", "IBM437", nil}
	ECIISO8859_1  = &ECI{1, "ISO8859_1", "ISO8859_1", []string{"ISO-8859-1"}}
	ECIISO8859_2  = &ECI{4, "ISO8859_2", "ISO8859_2", []string{"ISO-8859-2"}}
	ECIISO8859_3  = &ECI{5, "ISO8859_3", "ISO8859_3", []string{"ISO-8859-3"}}
	ECIISO8859_4  = &ECI{6, "ISO8859_4", "ISO8859_4", []string{"ISO-8859-4"}}
	ECIISO8859_5  = &ECI{7, "ISO8859_5", "ISO8859_5", []string{"ISO-8859-5"}}
	ECIISO8859_6  = &ECI{8, "ISO8859_6", "ISO8859_6", []string{"ISO-8859-6"}}
	ECIISO8859_7  = &ECI{9, "ISO8859_7", "ISO8859_7", []string{"ISO-8859-7"}}
	ECIISO8859_8  = &ECI{10, "ISO8859_8", "ISO8859_8", []string{"ISO-8859-8"}}
	ECIISO8859_9  = &ECI{11, "ISO8859_9", "ISO8859_9", []string{"ISO-8859-9"}}
	ECIISO8859_10 = &ECI{12, "ISO8859_10", "ISO8859_10", []string{"ISO-8859-10"}}
	ECIISO8859_11 = &ECI{13, "ISO8859_11", "ISO8859_11", []string{"ISO-8859-11"}}
	ECIISO8859_13 = &ECI{15, "ISO8859_13", "ISO8859_13", []string{"ISO-8859-13"}}
	ECIISO8859_14 = &ECI{16, "ISO8859_14", "ISO8859_14", []string{"ISO-8859-14"}}
	ECIISO8859_15 = &ECI{17, "ISO8859_15", "ISO8859_15", []string{"ISO-8859-15"}}
	ECIISO8859_16 = &ECI{18, "ISO8859_16", "ISO8859_16", []string{"ISO-8859-16"}}
	ECISJIS       = &ECI{20, "SJIS", "Shift_JIS", []string{"Shift_JIS"}}
	ECICp1250     = &ECI{21, "Cp1250", "Windows1250", []string{"windows-1250"}}
	ECICp1251     = &ECI{22, "Cp1251", "Windows1251", []string{"windows-1251"}}
	ECICp1252     = &ECI{23, "Cp1252", "Windows1252", []string{"windows-1252"}}
	ECICp1256     = &ECI{24, "Cp1256", "Windows1256", []string{"windows-1256"}}
	ECIUTF16BE    = &ECI{25, "UnicodeBigUnmarked", "UTF-16BE", []string{"UTF-16BE", "UnicodeBig"}}
	ECIUTF8       = &ECI{26, "UTF8", "UTF-8", []string{"UTF-8"}}
	ECIASCII      = &ECI{27, "ASCII", "US-ASCII", []string{"US-ASCII"}}
	ECIBig5       = &ECI{28, "Big5", "Big5", nil}
	ECIGB18030    = &ECI{29, "GB18030", "GB18030", []string{"GB2312", "EUC_CN", "GBK"}}
	ECIEUC_KR     = &ECI{30, "EUC_KR", "EUC-KR", []string{"EUC-KR"}}
)

var (
	valueToECI map[int]*ECI
	nameToECI  map[string]*ECI
)

func init() {
	valueToECI = make(map[int]*ECI)
	nameToECI = make(map[string]*ECI)

	allECIs := []*ECI{
		ECICp437, ECIISO8859_1, ECIISO8859_2, ECIISO8859_3, ECIISO8859_4,
		ECIISO8859_5, ECIISO8859_6, ECIISO8859_7, ECIISO8859_8, ECIISO8859_9,
		ECIISO8859_10, ECIISO8859_11, ECIISO8859_13, ECIISO8859_14,
		ECIISO8859_15, ECIISO8859_16, ECISJIS, ECICp1250, ECICp1251,
		ECICp1252, ECICp1256, ECIUTF16BE, ECIUTF8, ECIASCII, ECIBig5,
		ECIGB18030, ECIEUC_KR,
	}

	// Add additional value mappings
	extraValues := map[*ECI][]int{
		ECICp437:     {0, 2},
		ECIISO8859_1: {1, 3},
		ECIASCII:     {27, 170},
	}

	for _, eci := range allECIs {
		if vals, ok := extraValues[eci]; ok {
			for _, v := range vals {
				valueToECI[v] = eci
			}
		} else {
			valueToECI[eci.Value] = eci
		}
		nameToECI[eci.Name] = eci
		nameToECI[eci.GoName] = eci
		for _, alias := range eci.Aliases {
			nameToECI[alias] = eci
		}
	}
}

// GetECIByValue returns the ECI for the given value, or an error if invalid.
func GetECIByValue(value int) (*ECI, error) {
	if value < 0 || value >= 900 {
		return nil, ErrFormatECI
	}
	return valueToECI[value], nil
}

// GetECIByName returns the ECI for the given encoding name.
func GetECIByName(name string) *ECI {
	return nameToECI[name]
}
