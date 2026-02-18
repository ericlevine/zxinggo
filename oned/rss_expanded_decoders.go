package oned

import (
	"fmt"
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// parseExpandedInformation is the entry point: creates a decoder and parses.
func parseExpandedInformation(information *bitutil.BitArray) (string, error) {
	decoder := createExpandedDecoder(information)
	return decoder()
}

// createExpandedDecoder returns a closure that parses the expanded information.
func createExpandedDecoder(information *bitutil.BitArray) func() (string, error) {
	gd := &generalAppIdDecoder{information: information}

	if information.Get(1) {
		// AI01AndOtherAIs
		return func() (string, error) { return decodeAI01AndOtherAIs(gd) }
	}
	if !information.Get(2) {
		// AnyAIDecoder
		return func() (string, error) { return decodeAnyAI(gd) }
	}

	fourBit := extractNumericFromBitArray(information, 1, 4)
	switch fourBit {
	case 4:
		return func() (string, error) { return decodeAI013103(gd) }
	case 5:
		return func() (string, error) { return decodeAI01320x(gd) }
	}

	fiveBit := extractNumericFromBitArray(information, 1, 5)
	switch fiveBit {
	case 12:
		return func() (string, error) { return decodeAI01392x(gd) }
	case 13:
		return func() (string, error) { return decodeAI01393x(gd) }
	}

	sevenBit := extractNumericFromBitArray(information, 1, 7)
	switch sevenBit {
	case 56:
		return func() (string, error) { return decodeAI013x0x1x(gd, "310", "11") }
	case 57:
		return func() (string, error) { return decodeAI013x0x1x(gd, "320", "11") }
	case 58:
		return func() (string, error) { return decodeAI013x0x1x(gd, "310", "13") }
	case 59:
		return func() (string, error) { return decodeAI013x0x1x(gd, "320", "13") }
	case 60:
		return func() (string, error) { return decodeAI013x0x1x(gd, "310", "15") }
	case 61:
		return func() (string, error) { return decodeAI013x0x1x(gd, "320", "15") }
	case 62:
		return func() (string, error) { return decodeAI013x0x1x(gd, "310", "17") }
	case 63:
		return func() (string, error) { return decodeAI013x0x1x(gd, "320", "17") }
	}

	return func() (string, error) { return "", zxinggo.ErrFormat }
}

// --- GeneralAppIdDecoder ---

const (
	parsingStateNumeric = iota
	parsingStateAlpha
	parsingStateIsoIec646
)

type generalAppIdDecoder struct {
	information *bitutil.BitArray
	position    int
	encoding    int
	buf         strings.Builder
}

func (gd *generalAppIdDecoder) decodeAllCodes(buf *strings.Builder, initialPosition int) (string, error) {
	currentPosition := initialPosition
	var remaining *string
	for {
		newStr, newPos, hasRemaining, remainingValue, err := gd.decodeGeneralPurposeField(currentPosition, remaining)
		if err != nil {
			return "", err
		}
		parsedFields, err := parseFieldsInGeneralPurpose(newStr)
		if err != nil {
			return "", err
		}
		if parsedFields != "" {
			buf.WriteString(parsedFields)
		}
		if hasRemaining {
			s := fmt.Sprintf("%d", remainingValue)
			remaining = &s
		} else {
			remaining = nil
		}
		if currentPosition == newPos {
			break
		}
		currentPosition = newPos
	}
	return buf.String(), nil
}

func (gd *generalAppIdDecoder) decodeGeneralPurposeField(pos int, remaining *string) (string, int, bool, int, error) {
	gd.buf.Reset()
	if remaining != nil {
		gd.buf.WriteString(*remaining)
	}
	gd.position = pos
	// Note: encoding state is intentionally NOT reset here.
	// It persists across calls, matching Java's CurrentParsingState behavior
	// where only setPosition() is called, not setNumeric().

	info, err := gd.parseBlocks()
	if err != nil {
		return "", 0, false, 0, err
	}
	if info != nil && info.hasRemaining {
		return gd.buf.String(), gd.position, true, info.remainingValue, nil
	}
	return gd.buf.String(), gd.position, false, 0, nil
}

type decodedInfo struct {
	hasRemaining   bool
	remainingValue int
}

func (gd *generalAppIdDecoder) parseBlocks() (*decodedInfo, error) {
	var result *decodedInfo
	for {
		initialPosition := gd.position
		var finished bool
		var err error

		switch gd.encoding {
		case parsingStateAlpha:
			result, finished, err = gd.parseAlphaBlock()
		case parsingStateIsoIec646:
			result, finished, err = gd.parseIsoIec646Block()
		default:
			result, finished, err = gd.parseNumericBlock()
		}
		if err != nil {
			return nil, err
		}

		positionChanged := initialPosition != gd.position
		if !positionChanged && !finished {
			break
		}
		if finished {
			break
		}
	}
	return result, nil
}

func (gd *generalAppIdDecoder) parseNumericBlock() (*decodedInfo, bool, error) {
	for gd.isStillNumeric() {
		newPos, firstDigit, secondDigit, err := gd.decodeNumeric()
		if err != nil {
			return nil, false, err
		}
		gd.position = newPos

		if firstDigit == 10 { // FNC1
			if secondDigit == 10 {
				return &decodedInfo{}, true, nil
			}
			return &decodedInfo{hasRemaining: true, remainingValue: secondDigit}, true, nil
		}
		gd.buf.WriteByte(byte('0' + firstDigit))

		if secondDigit == 10 { // FNC1
			return &decodedInfo{}, true, nil
		}
		gd.buf.WriteByte(byte('0' + secondDigit))
	}

	if gd.isNumericToAlphaNumericLatch() {
		gd.encoding = parsingStateAlpha
		gd.position += 4
	}
	return nil, false, nil
}

func (gd *generalAppIdDecoder) parseAlphaBlock() (*decodedInfo, bool, error) {
	for gd.isStillAlpha() {
		newPos, ch, isFNC1 := gd.decodeAlphanumeric()
		gd.position = newPos
		if isFNC1 {
			return &decodedInfo{}, true, nil
		}
		gd.buf.WriteByte(ch)
	}

	if gd.isAlphaOr646ToNumericLatch() {
		gd.position += 3
		gd.encoding = parsingStateNumeric
	} else if gd.isAlphaTo646ToAlphaLatch() {
		if gd.position+5 < gd.information.Size() {
			gd.position += 5
		} else {
			gd.position = gd.information.Size()
		}
		gd.encoding = parsingStateIsoIec646
	}
	return nil, false, nil
}

func (gd *generalAppIdDecoder) parseIsoIec646Block() (*decodedInfo, bool, error) {
	for gd.isStillIsoIec646() {
		newPos, ch, isFNC1, err := gd.decodeIsoIec646()
		if err != nil {
			return nil, false, err
		}
		gd.position = newPos
		if isFNC1 {
			return &decodedInfo{}, true, nil
		}
		gd.buf.WriteByte(ch)
	}

	if gd.isAlphaOr646ToNumericLatch() {
		gd.position += 3
		gd.encoding = parsingStateNumeric
	} else if gd.isAlphaTo646ToAlphaLatch() {
		if gd.position+5 < gd.information.Size() {
			gd.position += 5
		} else {
			gd.position = gd.information.Size()
		}
		gd.encoding = parsingStateAlpha
	}
	return nil, false, nil
}

func (gd *generalAppIdDecoder) isStillNumeric() bool {
	pos := gd.position
	if pos+7 > gd.information.Size() {
		return pos+4 <= gd.information.Size()
	}
	for i := pos; i < pos+3; i++ {
		if gd.information.Get(i) {
			return true
		}
	}
	return gd.information.Get(pos + 3)
}

func (gd *generalAppIdDecoder) decodeNumeric() (newPos, firstDigit, secondDigit int, err error) {
	pos := gd.position
	if pos+7 > gd.information.Size() {
		numeric := gd.extractNumeric(pos, 4)
		if numeric == 0 {
			return gd.information.Size(), 10, 10, nil
		}
		return gd.information.Size(), numeric - 1, 10, nil
	}
	numeric := gd.extractNumeric(pos, 7)
	digit1 := (numeric - 8) / 11
	digit2 := (numeric - 8) % 11
	if digit1 < 0 || digit1 > 10 || digit2 < 0 || digit2 > 10 {
		return 0, 0, 0, zxinggo.ErrFormat
	}
	return pos + 7, digit1, digit2, nil
}

func (gd *generalAppIdDecoder) isStillAlpha() bool {
	pos := gd.position
	if pos+5 > gd.information.Size() {
		return false
	}
	fiveBitValue := gd.extractNumeric(pos, 5)
	if fiveBitValue >= 5 && fiveBitValue < 16 {
		return true
	}
	if pos+6 > gd.information.Size() {
		return false
	}
	sixBitValue := gd.extractNumeric(pos, 6)
	return sixBitValue >= 16 && sixBitValue < 63
}

func (gd *generalAppIdDecoder) decodeAlphanumeric() (newPos int, ch byte, isFNC1 bool) {
	pos := gd.position
	fiveBitValue := gd.extractNumeric(pos, 5)
	if fiveBitValue == 15 {
		return pos + 5, '$', true
	}
	if fiveBitValue >= 5 && fiveBitValue < 15 {
		return pos + 5, byte('0' + fiveBitValue - 5), false
	}
	sixBitValue := gd.extractNumeric(pos, 6)
	if sixBitValue >= 32 && sixBitValue < 58 {
		return pos + 6, byte(sixBitValue + 33), false
	}
	switch sixBitValue {
	case 58:
		return pos + 6, '*', false
	case 59:
		return pos + 6, ',', false
	case 60:
		return pos + 6, '-', false
	case 61:
		return pos + 6, '.', false
	case 62:
		return pos + 6, '/', false
	}
	// should not happen
	return pos + 6, '?', false
}

func (gd *generalAppIdDecoder) isStillIsoIec646() bool {
	pos := gd.position
	if pos+5 > gd.information.Size() {
		return false
	}
	fiveBitValue := gd.extractNumeric(pos, 5)
	if fiveBitValue >= 5 && fiveBitValue < 16 {
		return true
	}
	if pos+7 > gd.information.Size() {
		return false
	}
	sevenBitValue := gd.extractNumeric(pos, 7)
	if sevenBitValue >= 64 && sevenBitValue < 116 {
		return true
	}
	if pos+8 > gd.information.Size() {
		return false
	}
	eightBitValue := gd.extractNumeric(pos, 8)
	return eightBitValue >= 232 && eightBitValue < 253
}

func (gd *generalAppIdDecoder) decodeIsoIec646() (newPos int, ch byte, isFNC1 bool, err error) {
	pos := gd.position
	fiveBitValue := gd.extractNumeric(pos, 5)
	if fiveBitValue == 15 {
		return pos + 5, '$', true, nil
	}
	if fiveBitValue >= 5 && fiveBitValue < 15 {
		return pos + 5, byte('0' + fiveBitValue - 5), false, nil
	}
	sevenBitValue := gd.extractNumeric(pos, 7)
	if sevenBitValue >= 64 && sevenBitValue < 90 {
		return pos + 7, byte(sevenBitValue + 1), false, nil
	}
	if sevenBitValue >= 90 && sevenBitValue < 116 {
		return pos + 7, byte(sevenBitValue + 7), false, nil
	}
	eightBitValue := gd.extractNumeric(pos, 8)
	var c byte
	switch eightBitValue {
	case 232:
		c = '!'
	case 233:
		c = '"'
	case 234:
		c = '%'
	case 235:
		c = '&'
	case 236:
		c = '\''
	case 237:
		c = '('
	case 238:
		c = ')'
	case 239:
		c = '*'
	case 240:
		c = '+'
	case 241:
		c = ','
	case 242:
		c = '-'
	case 243:
		c = '.'
	case 244:
		c = '/'
	case 245:
		c = ':'
	case 246:
		c = ';'
	case 247:
		c = '<'
	case 248:
		c = '='
	case 249:
		c = '>'
	case 250:
		c = '?'
	case 251:
		c = '_'
	case 252:
		c = ' '
	default:
		return 0, 0, false, zxinggo.ErrFormat
	}
	return pos + 8, c, false, nil
}

func (gd *generalAppIdDecoder) isAlphaTo646ToAlphaLatch() bool {
	pos := gd.position
	if pos+1 > gd.information.Size() {
		return false
	}
	for i := 0; i < 5 && i+pos < gd.information.Size(); i++ {
		if i == 2 {
			if !gd.information.Get(pos + 2) {
				return false
			}
		} else if gd.information.Get(pos + i) {
			return false
		}
	}
	return true
}

func (gd *generalAppIdDecoder) isAlphaOr646ToNumericLatch() bool {
	pos := gd.position
	if pos+3 > gd.information.Size() {
		return false
	}
	for i := pos; i < pos+3; i++ {
		if gd.information.Get(i) {
			return false
		}
	}
	return true
}

func (gd *generalAppIdDecoder) isNumericToAlphaNumericLatch() bool {
	pos := gd.position
	if pos+1 > gd.information.Size() {
		return false
	}
	for i := 0; i < 4 && i+pos < gd.information.Size(); i++ {
		if gd.information.Get(pos + i) {
			return false
		}
	}
	return true
}

func (gd *generalAppIdDecoder) extractNumeric(pos, bits int) int {
	return extractNumericFromBitArray(gd.information, pos, bits)
}

func extractNumericFromBitArray(information *bitutil.BitArray, pos, bits int) int {
	value := 0
	for i := 0; i < bits; i++ {
		if information.Get(pos + i) {
			value |= 1 << uint(bits-i-1)
		}
	}
	return value
}

// --- AI01 decoder helpers ---

const gtinSize = 40

func encodeCompressedGtin(gd *generalAppIdDecoder, buf *strings.Builder, currentPos int) {
	buf.WriteString("(01)")
	initialPosition := buf.Len()
	buf.WriteByte('9')
	encodeCompressedGtinWithoutAI(gd, buf, currentPos, initialPosition)
}

func encodeCompressedGtinWithoutAI(gd *generalAppIdDecoder, buf *strings.Builder, currentPos, initialBufferPosition int) {
	for i := 0; i < 4; i++ {
		currentBlock := gd.extractNumeric(currentPos+10*i, 10)
		if currentBlock/100 == 0 {
			buf.WriteByte('0')
		}
		if currentBlock/10 == 0 {
			buf.WriteByte('0')
		}
		buf.WriteString(fmt.Sprintf("%d", currentBlock))
	}
	appendCheckDigit(buf, initialBufferPosition)
}

func appendCheckDigit(buf *strings.Builder, currentPos int) {
	s := buf.String()
	checkDigit := 0
	for i := 0; i < 13; i++ {
		digit := int(s[i+currentPos] - '0')
		if i&1 == 0 {
			checkDigit += 3 * digit
		} else {
			checkDigit += digit
		}
	}
	checkDigit = 10 - (checkDigit % 10)
	if checkDigit == 10 {
		checkDigit = 0
	}
	buf.WriteByte(byte('0' + checkDigit))
}

func encodeCompressedWeight(gd *generalAppIdDecoder, buf *strings.Builder, currentPos, weightSize int, addWeightCode func(*strings.Builder, int), checkWeight func(int) int) {
	originalWeightNumeric := gd.extractNumeric(currentPos, weightSize)
	addWeightCode(buf, originalWeightNumeric)
	weightNumeric := checkWeight(originalWeightNumeric)
	currentDivisor := 100000
	for i := 0; i < 5; i++ {
		if weightNumeric/currentDivisor == 0 {
			buf.WriteByte('0')
		}
		currentDivisor /= 10
	}
	buf.WriteString(fmt.Sprintf("%d", weightNumeric))
}

// --- Specific AI decoders ---

func decodeAI01AndOtherAIs(gd *generalAppIdDecoder) (string, error) {
	headerSize := 1 + 1 + 2
	var buf strings.Builder
	buf.WriteString("(01)")
	initialGtinPosition := buf.Len()
	firstGtinDigit := gd.extractNumeric(headerSize, 4)
	buf.WriteByte(byte('0' + firstGtinDigit))
	encodeCompressedGtinWithoutAI(gd, &buf, headerSize+4, initialGtinPosition)
	return gd.decodeAllCodes(&buf, headerSize+44)
}

func decodeAnyAI(gd *generalAppIdDecoder) (string, error) {
	headerSize := 2 + 1 + 2
	var buf strings.Builder
	return gd.decodeAllCodes(&buf, headerSize)
}

func decodeAI013103(gd *generalAppIdDecoder) (string, error) {
	headerSize := 4 + 1
	weightSize := 15
	if gd.information.Size() != headerSize+gtinSize+weightSize {
		return "", zxinggo.ErrNotFound
	}
	var buf strings.Builder
	encodeCompressedGtin(gd, &buf, headerSize)
	encodeCompressedWeight(gd, &buf, headerSize+gtinSize, weightSize,
		func(b *strings.Builder, weight int) { b.WriteString("(3103)") },
		func(weight int) int { return weight })
	return buf.String(), nil
}

func decodeAI01320x(gd *generalAppIdDecoder) (string, error) {
	headerSize := 4 + 1
	weightSize := 15
	if gd.information.Size() != headerSize+gtinSize+weightSize {
		return "", zxinggo.ErrNotFound
	}
	var buf strings.Builder
	encodeCompressedGtin(gd, &buf, headerSize)
	encodeCompressedWeight(gd, &buf, headerSize+gtinSize, weightSize,
		func(b *strings.Builder, weight int) {
			if weight < 10000 {
				b.WriteString("(3202)")
			} else {
				b.WriteString("(3203)")
			}
		},
		func(weight int) int {
			if weight < 10000 {
				return weight
			}
			return weight - 10000
		})
	return buf.String(), nil
}

func decodeAI01392x(gd *generalAppIdDecoder) (string, error) {
	headerSize := 5 + 1 + 2
	lastDigitSize := 2
	if gd.information.Size() < headerSize+gtinSize {
		return "", zxinggo.ErrNotFound
	}
	var buf strings.Builder
	encodeCompressedGtin(gd, &buf, headerSize)
	lastAIdigit := gd.extractNumeric(headerSize+gtinSize, lastDigitSize)
	buf.WriteString(fmt.Sprintf("(392%d)", lastAIdigit))
	info, newPos, _, _, err := gd.decodeGeneralPurposeField(headerSize+gtinSize+lastDigitSize, nil)
	if err != nil {
		return "", err
	}
	_ = newPos
	buf.WriteString(info)
	return buf.String(), nil
}

func decodeAI01393x(gd *generalAppIdDecoder) (string, error) {
	headerSize := 5 + 1 + 2
	lastDigitSize := 2
	firstThreeDigitsSize := 10
	if gd.information.Size() < headerSize+gtinSize {
		return "", zxinggo.ErrNotFound
	}
	var buf strings.Builder
	encodeCompressedGtin(gd, &buf, headerSize)
	lastAIdigit := gd.extractNumeric(headerSize+gtinSize, lastDigitSize)
	buf.WriteString(fmt.Sprintf("(393%d)", lastAIdigit))
	firstThreeDigits := gd.extractNumeric(headerSize+gtinSize+lastDigitSize, firstThreeDigitsSize)
	if firstThreeDigits/100 == 0 {
		buf.WriteByte('0')
	}
	if firstThreeDigits/10 == 0 {
		buf.WriteByte('0')
	}
	buf.WriteString(fmt.Sprintf("%d", firstThreeDigits))
	info, _, _, _, err := gd.decodeGeneralPurposeField(headerSize+gtinSize+lastDigitSize+firstThreeDigitsSize, nil)
	if err != nil {
		return "", err
	}
	buf.WriteString(info)
	return buf.String(), nil
}

func decodeAI013x0x1x(gd *generalAppIdDecoder, firstAIdigits, dateCode string) (string, error) {
	headerSize := 7 + 1
	weightSize := 20
	dateSize := 16
	if gd.information.Size() != headerSize+gtinSize+weightSize+dateSize {
		return "", zxinggo.ErrNotFound
	}
	var buf strings.Builder
	encodeCompressedGtin(gd, &buf, headerSize)
	encodeCompressedWeight(gd, &buf, headerSize+gtinSize, weightSize,
		func(b *strings.Builder, weight int) {
			b.WriteString(fmt.Sprintf("(%s%d)", firstAIdigits, weight/100000))
		},
		func(weight int) int { return weight % 100000 })
	encodeCompressedDate(&buf, gd, headerSize+gtinSize+weightSize, dateCode)
	return buf.String(), nil
}

func encodeCompressedDate(buf *strings.Builder, gd *generalAppIdDecoder, currentPos int, dateCode string) {
	numericDate := gd.extractNumeric(currentPos, 16)
	if numericDate == 38400 {
		return
	}
	buf.WriteByte('(')
	buf.WriteString(dateCode)
	buf.WriteByte(')')

	day := numericDate % 32
	numericDate /= 32
	month := numericDate%12 + 1
	numericDate /= 12
	year := numericDate

	if year/10 == 0 {
		buf.WriteByte('0')
	}
	buf.WriteString(fmt.Sprintf("%d", year))
	if month/10 == 0 {
		buf.WriteByte('0')
	}
	buf.WriteString(fmt.Sprintf("%d", month))
	if day/10 == 0 {
		buf.WriteByte('0')
	}
	buf.WriteString(fmt.Sprintf("%d", day))
}

// --- FieldParser ---

type dataLength struct {
	variable bool
	length   int
}

var twoDigitDataLength map[string]dataLength
var threeDigitDataLength map[string]dataLength
var threeDigitPlusDigitDataLength map[string]dataLength
var fourDigitDataLength map[string]dataLength

func init() {
	twoDigitDataLength = map[string]dataLength{
		"00": {false, 18}, "01": {false, 14}, "02": {false, 14},
		"10": {true, 20}, "11": {false, 6}, "12": {false, 6},
		"13": {false, 6}, "15": {false, 6}, "16": {false, 6},
		"17": {false, 6}, "20": {false, 2}, "21": {true, 20},
		"22": {true, 29}, "30": {true, 8}, "37": {true, 8},
	}
	for i := 90; i <= 99; i++ {
		twoDigitDataLength[fmt.Sprintf("%d", i)] = dataLength{true, 30}
	}

	threeDigitDataLength = map[string]dataLength{
		"235": {true, 28}, "240": {true, 30}, "241": {true, 30},
		"242": {true, 6}, "243": {true, 20}, "250": {true, 30},
		"251": {true, 30}, "253": {true, 30}, "254": {true, 20},
		"255": {true, 25}, "400": {true, 30}, "401": {true, 30},
		"402": {false, 17}, "403": {true, 30},
		"410": {false, 13}, "411": {false, 13}, "412": {false, 13},
		"413": {false, 13}, "414": {false, 13}, "415": {false, 13},
		"416": {false, 13}, "417": {false, 13},
		"420": {true, 20}, "421": {true, 15}, "422": {false, 3},
		"423": {true, 15}, "424": {false, 3}, "425": {true, 15},
		"426": {false, 3}, "427": {true, 3},
		"710": {true, 20}, "711": {true, 20}, "712": {true, 20},
		"713": {true, 20}, "714": {true, 20}, "715": {true, 20},
	}

	threeDigitPlusDigitDataLength = map[string]dataLength{}
	for i := 310; i <= 316; i++ {
		threeDigitPlusDigitDataLength[fmt.Sprintf("%d", i)] = dataLength{false, 6}
	}
	for i := 320; i <= 337; i++ {
		threeDigitPlusDigitDataLength[fmt.Sprintf("%d", i)] = dataLength{false, 6}
	}
	for i := 340; i <= 357; i++ {
		threeDigitPlusDigitDataLength[fmt.Sprintf("%d", i)] = dataLength{false, 6}
	}
	for i := 360; i <= 369; i++ {
		threeDigitPlusDigitDataLength[fmt.Sprintf("%d", i)] = dataLength{false, 6}
	}
	threeDigitPlusDigitDataLength["390"] = dataLength{true, 15}
	threeDigitPlusDigitDataLength["391"] = dataLength{true, 18}
	threeDigitPlusDigitDataLength["392"] = dataLength{true, 15}
	threeDigitPlusDigitDataLength["393"] = dataLength{true, 18}
	threeDigitPlusDigitDataLength["394"] = dataLength{false, 4}
	threeDigitPlusDigitDataLength["395"] = dataLength{false, 6}
	threeDigitPlusDigitDataLength["703"] = dataLength{true, 30}
	threeDigitPlusDigitDataLength["723"] = dataLength{true, 30}

	fourDigitDataLength = map[string]dataLength{
		"4300": {true, 35}, "4301": {true, 35}, "4302": {true, 70},
		"4303": {true, 70}, "4304": {true, 70}, "4305": {true, 70},
		"4306": {true, 70}, "4307": {false, 2}, "4308": {true, 30},
		"4309": {false, 20}, "4310": {true, 35}, "4311": {true, 35},
		"4312": {true, 70}, "4313": {true, 70}, "4314": {true, 70},
		"4315": {true, 70}, "4316": {true, 70}, "4317": {false, 2},
		"4318": {true, 20}, "4319": {true, 30}, "4320": {true, 35},
		"4321": {false, 1}, "4322": {false, 1}, "4323": {false, 1},
		"4324": {false, 10}, "4325": {false, 10}, "4326": {false, 6},
		"7001": {false, 13}, "7002": {true, 30}, "7003": {false, 10},
		"7004": {true, 4}, "7005": {true, 12}, "7006": {false, 6},
		"7007": {true, 12}, "7008": {true, 3}, "7009": {true, 10},
		"7010": {true, 2}, "7011": {true, 10},
		"7020": {true, 20}, "7021": {true, 20}, "7022": {true, 20},
		"7023": {true, 30}, "7040": {false, 4}, "7240": {true, 20},
		"8001": {false, 14}, "8002": {true, 20}, "8003": {true, 30},
		"8004": {true, 30}, "8005": {false, 6}, "8006": {false, 18},
		"8007": {true, 34}, "8008": {true, 12}, "8009": {true, 50},
		"8010": {true, 30}, "8011": {true, 12}, "8012": {true, 20},
		"8013": {true, 25}, "8017": {false, 18}, "8018": {false, 18},
		"8019": {true, 10}, "8020": {true, 25}, "8026": {false, 18},
		"8100": {false, 6}, "8101": {false, 10}, "8102": {false, 2},
		"8110": {true, 70}, "8111": {false, 4}, "8112": {true, 70},
		"8200": {true, 70},
	}
}

func parseFieldsInGeneralPurpose(rawInformation string) (string, error) {
	if rawInformation == "" {
		return "", nil
	}
	if len(rawInformation) < 2 {
		return "", zxinggo.ErrNotFound
	}

	// 2-digit AI
	if dl, ok := twoDigitDataLength[rawInformation[:2]]; ok {
		if dl.variable {
			return processVariableAI(2, dl.length, rawInformation)
		}
		return processFixedAI(2, dl.length, rawInformation)
	}

	if len(rawInformation) < 3 {
		return "", zxinggo.ErrNotFound
	}

	// 3-digit AI
	first3 := rawInformation[:3]
	if dl, ok := threeDigitDataLength[first3]; ok {
		if dl.variable {
			return processVariableAI(3, dl.length, rawInformation)
		}
		return processFixedAI(3, dl.length, rawInformation)
	}

	if len(rawInformation) < 4 {
		return "", zxinggo.ErrNotFound
	}

	// 3-digit+digit AI
	if dl, ok := threeDigitPlusDigitDataLength[first3]; ok {
		if dl.variable {
			return processVariableAI(4, dl.length, rawInformation)
		}
		return processFixedAI(4, dl.length, rawInformation)
	}

	// 4-digit AI
	if dl, ok := fourDigitDataLength[rawInformation[:4]]; ok {
		if dl.variable {
			return processVariableAI(4, dl.length, rawInformation)
		}
		return processFixedAI(4, dl.length, rawInformation)
	}

	return "", zxinggo.ErrNotFound
}

func processFixedAI(aiSize, fieldSize int, rawInformation string) (string, error) {
	if len(rawInformation) < aiSize {
		return "", zxinggo.ErrNotFound
	}
	ai := rawInformation[:aiSize]
	if len(rawInformation) < aiSize+fieldSize {
		return "", zxinggo.ErrNotFound
	}
	field := rawInformation[aiSize : aiSize+fieldSize]
	remaining := rawInformation[aiSize+fieldSize:]
	result := "(" + ai + ")" + field
	parsedAI, err := parseFieldsInGeneralPurpose(remaining)
	if err != nil {
		return "", err
	}
	if parsedAI == "" {
		return result, nil
	}
	return result + parsedAI, nil
}

func processVariableAI(aiSize, variableFieldSize int, rawInformation string) (string, error) {
	ai := rawInformation[:aiSize]
	maxSize := aiSize + variableFieldSize
	if maxSize > len(rawInformation) {
		maxSize = len(rawInformation)
	}
	field := rawInformation[aiSize:maxSize]
	remaining := rawInformation[maxSize:]
	result := "(" + ai + ")" + field
	parsedAI, err := parseFieldsInGeneralPurpose(remaining)
	if err != nil {
		return "", err
	}
	if parsedAI == "" {
		return result, nil
	}
	return result + parsedAI, nil
}

