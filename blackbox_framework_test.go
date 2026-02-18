package zxinggo_test

import (
	"bufio"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/binarizer"
	"github.com/ericlevine/zxinggo/pdf417"
)

// blackboxTestDir is the path to the blackbox test resources (copied from Java ZXing).
const blackboxTestDir = "testdata/blackbox"

// blackboxTestRotation defines expected pass/fail thresholds for one rotation angle.
type blackboxTestRotation struct {
	rotation             float64
	mustPassCount        int
	tryHarderCount       int
	maxMisreads          int
	maxTryHarderMisreads int
}

// blackboxTestCase defines a complete blackbox test for one format/directory.
type blackboxTestCase struct {
	dir    string // subdirectory name under blackboxTestDir, e.g. "aztec-1"
	format zxinggo.Format
	tests  []blackboxTestRotation
	opts   *zxinggo.DecodeOptions // optional extra decode options
}

// rotateImage rotates an image by the given degrees (must be a multiple of 90).
func rotateImage(img image.Image, degrees float64) image.Image {
	switch int(degrees) % 360 {
	case 0:
		return img
	case 90:
		return rotate90(img)
	case 180:
		return rotate180(img)
	case 270:
		return rotate270(img)
	default:
		panic(fmt.Sprintf("unsupported rotation: %v degrees", degrees))
	}
}

func rotate90(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(h-1-y, x, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func rotate180(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(w-1-x, h-1-y, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func rotate270(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(y, w-1-x, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// loadExpectedText loads expected barcode text from a .txt or .bin file.
func loadExpectedText(basePath string) (string, error) {
	// Try .txt first (UTF-8)
	txtPath := basePath + ".txt"
	if data, err := os.ReadFile(txtPath); err == nil {
		return string(data), nil
	}

	// Try .bin (ISO-8859-1 / Latin-1)
	binPath := basePath + ".bin"
	data, err := os.ReadFile(binPath)
	if err != nil {
		return "", fmt.Errorf("no expected text file found for %s (.txt or .bin)", basePath)
	}
	// Convert ISO-8859-1 to UTF-8
	runes := make([]rune, len(data))
	for i, b := range data {
		runes[i] = rune(b)
	}
	return string(runes), nil
}

// loadExpectedMetadata loads expected metadata from a .metadata.txt file.
// Returns nil if the file doesn't exist.
func loadExpectedMetadata(basePath string) map[string]string {
	metaPath := basePath + ".metadata.txt"
	f, err := os.Open(metaPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	metadata := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			metadata[parts[0]] = parts[1]
		}
	}
	return metadata
}

// metadataKeyFromString converts a string metadata key name to ResultMetadataKey.
func metadataKeyFromString(name string) (zxinggo.ResultMetadataKey, bool) {
	switch name {
	case "ORIENTATION":
		return zxinggo.MetadataOrientation, true
	case "BYTE_SEGMENTS":
		return zxinggo.MetadataByteSegments, true
	case "ERROR_CORRECTION_LEVEL":
		return zxinggo.MetadataErrorCorrectionLevel, true
	case "ERRORS_CORRECTED":
		return zxinggo.MetadataErrorsCorrected, true
	case "ERASURES_CORRECTED":
		return zxinggo.MetadataErasuresCorrected, true
	case "ISSUE_NUMBER":
		return zxinggo.MetadataIssueNumber, true
	case "SUGGESTED_PRICE":
		return zxinggo.MetadataSuggestedPrice, true
	case "POSSIBLE_COUNTRY":
		return zxinggo.MetadataPossibleCountry, true
	case "UPC_EAN_EXTENSION":
		return zxinggo.MetadataUPCEANExtension, true
	case "PDF417_EXTRA_METADATA":
		return zxinggo.MetadataPDF417ExtraMetadata, true
	case "STRUCTURED_APPEND_SEQUENCE":
		return zxinggo.MetadataStructuredAppendSequence, true
	case "STRUCTURED_APPEND_PARITY":
		return zxinggo.MetadataStructuredAppendParity, true
	case "SYMBOLOGY_IDENTIFIER":
		return zxinggo.MetadataSymbologyIdentifier, true
	default:
		return zxinggo.MetadataOther, false
	}
}

// checkMetadata verifies that a decode result contains the expected metadata.
func checkMetadata(result *zxinggo.Result, expectedMeta map[string]string) bool {
	if len(expectedMeta) == 0 {
		return true
	}
	for keyName, expectedVal := range expectedMeta {
		key, ok := metadataKeyFromString(keyName)
		if !ok {
			continue // skip unknown metadata keys
		}
		actual, exists := result.Metadata[key]
		if !exists {
			return false
		}
		// Compare as strings - convert actual value
		actualStr := fmt.Sprintf("%v", actual)
		if actualStr != expectedVal {
			return false
		}
	}
	return true
}

// imageExtensions are the file extensions to look for in test directories.
var imageExtensions = []string{".png", ".jpg", ".jpeg", ".gif"}

// findImageFiles finds all image files in a directory.
func findImageFiles(dir string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		for _, ie := range imageExtensions {
			if ext == ie {
				files = append(files, filepath.Join(dir, entry.Name()))
				break
			}
		}
	}
	return files, nil
}

type imageTestData struct {
	path         string
	expectedText string
	metadata     map[string]string
}

// runBlackBoxTest runs a complete blackbox test for a given test case.
func runBlackBoxTest(t *testing.T, tc blackboxTestCase) {
	t.Helper()

	dir := filepath.Join(blackboxTestDir, tc.dir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skipf("test directory %s not found, skipping", dir)
		return
	}

	imageFiles, err := findImageFiles(dir)
	if err != nil {
		t.Fatalf("failed to find image files in %s: %v", dir, err)
	}
	if len(imageFiles) == 0 {
		t.Fatalf("no image files found in %s", dir)
	}

	// Load all test data
	var testData []imageTestData
	for _, imgPath := range imageFiles {
		ext := filepath.Ext(imgPath)
		basePath := imgPath[:len(imgPath)-len(ext)]

		expectedText, err := loadExpectedText(basePath)
		if err != nil {
			t.Logf("skipping %s: %v", filepath.Base(imgPath), err)
			continue
		}

		metadata := loadExpectedMetadata(basePath)
		testData = append(testData, imageTestData{
			path:         imgPath,
			expectedText: expectedText,
			metadata:     metadata,
		})
	}

	if len(testData) == 0 {
		t.Fatalf("no valid test images found in %s", dir)
	}

	testCount := len(tc.tests)
	passedCounts := make([]int, testCount)
	misreadCounts := make([]int, testCount)
	tryHarderCounts := make([]int, testCount)
	tryHarderMisreadCounts := make([]int, testCount)

	for _, td := range testData {
		// Load image
		f, err := os.Open(td.path)
		if err != nil {
			t.Fatalf("failed to open %s: %v", td.path, err)
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			t.Logf("failed to decode image %s: %v", filepath.Base(td.path), err)
			continue
		}

		for i, rot := range tc.tests {
			rotated := rotateImage(img, rot.rotation)

			// Normal decode (no TryHarder)
			source := zxinggo.NewImageLuminanceSource(rotated)
			bitmap := zxinggo.NewBinaryBitmap(binarizer.NewHybrid(source))
			result := tryDecode(bitmap, tc.format, false, tc.opts)
			outcome := classifyResult(result, tc.format, td.expectedText, td.metadata)
			switch outcome {
			case resultPassed:
				passedCounts[i]++
			case resultMisread:
				misreadCounts[i]++
				t.Logf("  MISREAD rot=%.0f file=%s got=%q expected=%q format=%v meta=%v",
					rot.rotation, filepath.Base(td.path),
					resultText(result), td.expectedText, result.Format, result.Metadata)
			case resultNotFound:
				t.Logf("  NOTFOUND rot=%.0f file=%s", rot.rotation, filepath.Base(td.path))
			}

			// TryHarder decode
			source2 := zxinggo.NewImageLuminanceSource(rotated)
			bitmap2 := zxinggo.NewBinaryBitmap(binarizer.NewHybrid(source2))
			result2 := tryDecode(bitmap2, tc.format, true, tc.opts)
			outcome2 := classifyResult(result2, tc.format, td.expectedText, td.metadata)
			switch outcome2 {
			case resultPassed:
				tryHarderCounts[i]++
			case resultMisread:
				tryHarderMisreadCounts[i]++
				t.Logf("  MISREAD(TH) rot=%.0f file=%s got=%q expected=%q format=%v meta=%v",
					rot.rotation, filepath.Base(td.path),
					resultText(result2), td.expectedText, result2.Format, result2.Metadata)
			case resultNotFound:
				t.Logf("  NOTFOUND(TH) rot=%.0f file=%s", rot.rotation, filepath.Base(td.path))
			}
		}
	}

	// Log results
	totalFound := 0
	totalMustPass := 0
	totalMisread := 0
	totalMaxMisread := 0
	for i, rot := range tc.tests {
		t.Logf("Rotation %3.0f°: %d/%d passed (need %d), %d misread (max %d) | TryHarder: %d/%d passed (need %d), %d misread (max %d)",
			rot.rotation,
			passedCounts[i], len(testData), rot.mustPassCount, misreadCounts[i], rot.maxMisreads,
			tryHarderCounts[i], len(testData), rot.tryHarderCount, tryHarderMisreadCounts[i], rot.maxTryHarderMisreads)

		totalFound += passedCounts[i] + tryHarderCounts[i]
		totalMustPass += rot.mustPassCount + rot.tryHarderCount
		totalMisread += misreadCounts[i] + tryHarderMisreadCounts[i]
		totalMaxMisread += rot.maxMisreads + rot.maxTryHarderMisreads
	}

	t.Logf("Total: %d found of %d needed, %d misread of %d max",
		totalFound, totalMustPass, totalMisread, totalMaxMisread)

	if totalFound > totalMustPass {
		t.Logf("+++ Test too lax by %d images", totalFound-totalMustPass)
	}

	// Assert thresholds
	for i, rot := range tc.tests {
		if passedCounts[i] < rot.mustPassCount {
			t.Errorf("Rotation %.0f°: Too many images failed: got %d, need %d",
				rot.rotation, passedCounts[i], rot.mustPassCount)
		}
		if tryHarderCounts[i] < rot.tryHarderCount {
			t.Errorf("Rotation %.0f° (TryHarder): Too many images failed: got %d, need %d",
				rot.rotation, tryHarderCounts[i], rot.tryHarderCount)
		}
		if misreadCounts[i] > rot.maxMisreads {
			t.Errorf("Rotation %.0f°: Too many misreads: got %d, max %d",
				rot.rotation, misreadCounts[i], rot.maxMisreads)
		}
		if tryHarderMisreadCounts[i] > rot.maxTryHarderMisreads {
			t.Errorf("Rotation %.0f° (TryHarder): Too many misreads: got %d, max %d",
				rot.rotation, tryHarderMisreadCounts[i], rot.maxTryHarderMisreads)
		}
	}
}

type decodeOutcome int

const (
	resultNotFound decodeOutcome = iota
	resultPassed
	resultMisread
)

func resultText(r *zxinggo.Result) string {
	if r == nil {
		return "<nil>"
	}
	return r.Text
}

// classifyResult classifies a decode result as passed, misread, or not found.
func classifyResult(result *zxinggo.Result, format zxinggo.Format, expectedText string, expectedMeta map[string]string) decodeOutcome {
	if result == nil {
		return resultNotFound
	}
	if result.Format != format {
		return resultMisread
	}
	if result.Text != expectedText {
		return resultMisread
	}
	if !checkMetadata(result, expectedMeta) {
		return resultMisread
	}
	return resultPassed
}

// tryDecode attempts to decode a barcode, trying PureBarcode first then normal.
// Recovers from panics in decoders to prevent one bad image from crashing the entire test.
func tryDecode(bitmap *zxinggo.BinaryBitmap, format zxinggo.Format, tryHarder bool, extraOpts *zxinggo.DecodeOptions) (result *zxinggo.Result) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
		}
	}()

	opts := &zxinggo.DecodeOptions{
		PossibleFormats: []zxinggo.Format{format},
		TryHarder:       tryHarder,
		PureBarcode:     true,
	}
	if extraOpts != nil {
		opts.AlsoInverted = extraOpts.AlsoInverted
		opts.AllowedEANExtensions = extraOpts.AllowedEANExtensions
	}

	// Try PureBarcode first (like Java)
	result, err := zxinggo.Decode(bitmap, opts)
	if err == nil {
		return result
	}

	// Fall back to normal decode
	opts2 := &zxinggo.DecodeOptions{
		PossibleFormats: []zxinggo.Format{format},
		TryHarder:       tryHarder,
	}
	if extraOpts != nil {
		opts2.AlsoInverted = extraOpts.AlsoInverted
		opts2.AllowedEANExtensions = extraOpts.AllowedEANExtensions
	}
	result, err = zxinggo.Decode(bitmap, opts2)
	if err == nil {
		return result
	}

	return nil
}

// Helper to create test rotation with just pass counts (maxMisreads=0)
func rot(degrees float64, mustPass, tryHarderPass int) blackboxTestRotation {
	return blackboxTestRotation{
		rotation:      degrees,
		mustPassCount: mustPass,
		tryHarderCount: tryHarderPass,
	}
}

// Helper to create test rotation with misread allowances
func rotM(degrees float64, mustPass, tryHarderPass, maxMisreads, maxTryHarderMisreads int) blackboxTestRotation {
	return blackboxTestRotation{
		rotation:             degrees,
		mustPassCount:        mustPass,
		tryHarderCount:       tryHarderPass,
		maxMisreads:          maxMisreads,
		maxTryHarderMisreads: maxTryHarderMisreads,
	}
}

// runPDF417MultiTest runs a Macro PDF417 multi-symbol test.
// Images are grouped by base name (e.g., 01-01.png, 01-02.png -> group "01").
// Each group's images are decoded separately, results sorted by segment index,
// then concatenated text is compared to the expected text.
func runPDF417MultiTest(t *testing.T, dir string, mustPass int) {
	t.Helper()

	testDir := filepath.Join(blackboxTestDir, dir)
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Skipf("test directory %s not found, skipping", testDir)
		return
	}

	// Group image files by base name (before the dash)
	entries, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("failed to read directory %s: %v", testDir, err)
	}

	type imageGroup struct {
		baseName     string
		expectedText string
		imageFiles   []string
	}

	groupMap := make(map[string]*imageGroup)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		isImage := false
		for _, ie := range imageExtensions {
			if ext == ie {
				isImage = true
				break
			}
		}
		if !isImage {
			continue
		}
		// Extract base name: e.g., "01-01.png" -> "01"
		base := name[:len(name)-len(ext)]
		dashIdx := strings.Index(base, "-")
		if dashIdx < 0 {
			continue // not a multi-part image
		}
		groupName := base[:dashIdx]
		g, ok := groupMap[groupName]
		if !ok {
			g = &imageGroup{baseName: groupName}
			groupMap[groupName] = g
		}
		g.imageFiles = append(g.imageFiles, filepath.Join(testDir, name))
	}

	// Load expected text for each group
	var groups []*imageGroup
	for _, g := range groupMap {
		text, err := loadExpectedText(filepath.Join(testDir, g.baseName))
		if err != nil {
			t.Logf("skipping group %s: %v", g.baseName, err)
			continue
		}
		g.expectedText = text
		groups = append(groups, g)
	}

	if len(groups) == 0 {
		t.Fatalf("no valid test groups found in %s", testDir)
	}

	passed := 0
	for _, g := range groups {
		// Decode all images in the group
		var allResults []*zxinggo.Result
		for _, imgPath := range g.imageFiles {
			f, err := os.Open(imgPath)
			if err != nil {
				t.Logf("failed to open %s: %v", imgPath, err)
				continue
			}
			img, _, err := image.Decode(f)
			f.Close()
			if err != nil {
				t.Logf("failed to decode image %s: %v", imgPath, err)
				continue
			}

			source := zxinggo.NewImageLuminanceSource(img)
			bitmap := zxinggo.NewBinaryBitmap(binarizer.NewHybrid(source))
			opts := &zxinggo.DecodeOptions{
				PossibleFormats: []zxinggo.Format{zxinggo.FormatPDF417},
			}
			results, err := pdf417.NewPDF417Reader().DecodeMultiple(bitmap, opts)
			if err != nil {
				continue
			}
			allResults = append(allResults, results...)
		}

		if len(allResults) == 0 {
			t.Logf("group %s: no barcodes decoded", g.baseName)
			continue
		}

		// Sort by segment index
		sortPDF417ResultsBySegment(allResults)

		// Concatenate text
		var combined strings.Builder
		for _, r := range allResults {
			combined.WriteString(r.Text)
		}

		if combined.String() == g.expectedText {
			passed++
		} else {
			t.Logf("group %s: text mismatch: got %q, want %q", g.baseName, combined.String(), g.expectedText)
		}
	}

	t.Logf("PDF417 multi-symbol: %d/%d passed (need %d)", passed, len(groups), mustPass)
	if passed < mustPass {
		t.Errorf("too few groups passed: got %d, need %d", passed, mustPass)
	}
}

func sortPDF417ResultsBySegment(results []*zxinggo.Result) {
	sort.Slice(results, func(i, j int) bool {
		return pdf417SegmentIndex(results[i]) < pdf417SegmentIndex(results[j])
	})
}

func pdf417SegmentIndex(r *zxinggo.Result) int {
	meta, ok := r.Metadata[zxinggo.MetadataPDF417ExtraMetadata]
	if !ok {
		return 0
	}
	// Use reflect to access SegmentIndex field on the concrete type
	v := reflect.ValueOf(meta)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		f := v.FieldByName("SegmentIndex")
		if f.IsValid() {
			return int(f.Int())
		}
	}
	return 0
}

