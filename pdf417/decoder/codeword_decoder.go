package decoder

import "math"

// ratiosTable is the pre-computed symbol ratio table used by the codeword decoder.
var ratiosTable [len(symbolTable)][barsInModule]float32

func init() {
	for i := 0; i < len(symbolTable); i++ {
		currentSymbol := symbolTable[i]
		currentBit := currentSymbol & 0x1
		for j := 0; j < barsInModule; j++ {
			var size float32
			for (currentSymbol & 0x1) == currentBit {
				size += 1.0
				currentSymbol >>= 1
			}
			currentBit = currentSymbol & 0x1
			ratiosTable[i][barsInModule-j-1] = size / float32(modulesInCodeword)
		}
	}
}

// GetDecodedValue decodes a module bit count pattern into a codeword value.
func GetDecodedValue(moduleBitCount []int) int {
	decodedValue := getDecodedCodewordValue(sampleBitCounts(moduleBitCount))
	if decodedValue != -1 {
		return decodedValue
	}
	return getClosestDecodedValue(moduleBitCount)
}

func sampleBitCounts(moduleBitCount []int) []int {
	bitCountSum := sumInts(moduleBitCount)
	result := make([]int, barsInModule)
	bitCountIndex := 0
	sumPreviousBits := 0
	for i := 0; i < modulesInCodeword; i++ {
		sampleIndex := float64(bitCountSum)/(2.0*float64(modulesInCodeword)) +
			float64(i)*float64(bitCountSum)/float64(modulesInCodeword)
		if float64(sumPreviousBits+moduleBitCount[bitCountIndex]) <= sampleIndex {
			sumPreviousBits += moduleBitCount[bitCountIndex]
			bitCountIndex++
		}
		result[bitCountIndex]++
	}
	return result
}

func getDecodedCodewordValue(moduleBitCount []int) int {
	decodedValue := getBitValue(moduleBitCount)
	if getCodeword(decodedValue) == -1 {
		return -1
	}
	return decodedValue
}

func getBitValue(moduleBitCount []int) int {
	var result int64
	for i := 0; i < len(moduleBitCount); i++ {
		for bit := 0; bit < moduleBitCount[i]; bit++ {
			result = (result << 1)
			if i%2 == 0 {
				result |= 1
			}
		}
	}
	return int(result)
}

func getClosestDecodedValue(moduleBitCount []int) int {
	bitCountSum := sumInts(moduleBitCount)
	bitCountRatios := make([]float32, barsInModule)
	if bitCountSum > 1 {
		for i := 0; i < len(bitCountRatios); i++ {
			bitCountRatios[i] = float32(moduleBitCount[i]) / float32(bitCountSum)
		}
	}
	bestMatchError := float32(math.MaxFloat32)
	bestMatch := -1
	for j := 0; j < len(ratiosTable); j++ {
		var errorVal float32
		ratioTableRow := ratiosTable[j]
		for k := 0; k < barsInModule; k++ {
			diff := ratioTableRow[k] - bitCountRatios[k]
			errorVal += diff * diff
			if errorVal >= bestMatchError {
				break
			}
		}
		if errorVal < bestMatchError {
			bestMatchError = errorVal
			bestMatch = symbolTable[j]
		}
	}
	return bestMatch
}

// sumInts returns the sum of elements in an int slice.
func sumInts(values []int) int {
	sum := 0
	for _, v := range values {
		sum += v
	}
	return sum
}
