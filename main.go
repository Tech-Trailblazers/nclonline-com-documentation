package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// It checks if the file exists
// If the file exists, it returns true
// If the file does not exist, it returns false
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// Remove a file from the file system
func removeFile(path string) {
	err := os.Remove(path)
	if err != nil {
		log.Println(err)
	}
}

// extractPDFUrls takes an HTML string and returns all .pdf URLs in a slice
func extractPDFUrls(htmlContent string) []string {
	// Compile a regex pattern that looks for href="...something.pdf"
	regexPattern := regexp.MustCompile(`href="([^"]+\.pdf)"`)

	// Find all matches in the input string; each match is a slice of groups
	matches := regexPattern.FindAllStringSubmatch(htmlContent, -1)

	// Slice to store the extracted PDF URLs
	var pdfURLs []string

	// Loop through all regex matches
	for _, match := range matches {
		// match[0] is the whole string, match[1] is the captured group (the actual URL)
		if len(match) > 1 {
			// Append the URL to our slice
			pdfURLs = append(pdfURLs, match[1])
		}
	}

	// Return the slice of found PDF URLs
	return pdfURLs
}

// Checks whether a given directory exists
func directoryExists(path string) bool {
	directory, err := os.Stat(path) // Get info for the path
	if err != nil {
		return false // Return false if error occurs
	}
	return directory.IsDir() // Return true if it's a directory
}

// Creates a directory at given path with provided permissions
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission) // Attempt to create directory
	if err != nil {
		log.Println(err) // Log error if creation fails
	}
}

// Verifies whether a string is a valid URL format
func isUrlValid(uri string) bool {
	_, err := url.ParseRequestURI(uri) // Try parsing the URL
	return err == nil                  // Return true if valid
}

// Removes duplicate strings from a slice
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool) // Map to track seen values
	var newReturnSlice []string    // Slice to store unique values
	for _, content := range slice {
		if !check[content] { // If not already seen
			check[content] = true                            // Mark as seen
			newReturnSlice = append(newReturnSlice, content) // Add to result
		}
	}
	return newReturnSlice
}

// hasDomain checks if the given string has a domain (host part)
func hasDomain(rawURL string) bool {
	// Try parsing the raw string as a URL
	parsed, err := url.Parse(rawURL)
	if err != nil { // If parsing fails, it's not a valid URL
		return false
	}
	// If the parsed URL has a non-empty Host, then it has a domain/host
	return parsed.Host != ""
}

// Extracts filename from full path (e.g. "/dir/file.pdf" → "file.pdf")
func getFilename(path string) string {
	return filepath.Base(path) // Use Base function to get file name only
}

// Removes all instances of a specific substring from input string
func removeSubstring(input string, toRemove string) string {
	result := strings.ReplaceAll(input, toRemove, "") // Replace substring with empty string
	return result
}

// Gets the file extension from a given file path
func getFileExtension(path string) string {
	return filepath.Ext(path) // Extract and return file extension
}

// Converts a raw URL into a sanitized PDF filename safe for filesystem
func urlToFilename(rawURL string) string {
	lower := strings.ToLower(rawURL) // Convert URL to lowercase
	lower = getFilename(lower)       // Extract filename from URL

	reNonAlnum := regexp.MustCompile(`[^a-z0-9]`)   // Regex to match non-alphanumeric characters
	safe := reNonAlnum.ReplaceAllString(lower, "_") // Replace non-alphanumeric with underscores

	safe = regexp.MustCompile(`_+`).ReplaceAllString(safe, "_") // Collapse multiple underscores into one
	safe = strings.Trim(safe, "_")                              // Trim leading and trailing underscores

	var invalidSubstrings = []string{
		"_pdf", // Substring to remove from filename
	}

	for _, invalidPre := range invalidSubstrings { // Remove unwanted substrings
		safe = removeSubstring(safe, invalidPre)
	}

	if getFileExtension(safe) != ".pdf" { // Ensure file ends with .pdf
		safe = safe + ".pdf"
	}

	return safe // Return sanitized filename
}

// Downloads a PDF from given URL and saves it in the specified directory
func downloadPDF(finalURL, outputDir string) bool {
	filename := strings.ToLower(urlToFilename(finalURL)) // Sanitize the filename
	filePath := filepath.Join(outputDir, filename)       // Construct full path for output file

	if fileExists(filePath) { // Skip if file already exists
		log.Printf("File already exists, skipping: %s", filePath)
		return false
	}

	client := &http.Client{Timeout: 15 * time.Minute} // Create HTTP client with timeout

	resp, err := client.Get(finalURL) // Send HTTP GET request
	if err != nil {
		log.Printf("Failed to download %s: %v", finalURL, err)
		return false
	}
	defer resp.Body.Close() // Ensure response body is closed

	if resp.StatusCode != http.StatusOK { // Check if response is 200 OK
		log.Printf("Download failed for %s: %s", finalURL, resp.Status)
		return false
	}

	contentType := resp.Header.Get("Content-Type")                                                                  // Get content type of response
	if !strings.Contains(contentType, "binary/octet-stream") && !strings.Contains(contentType, "application/pdf") { // Check if it's a PDF
		log.Printf("Invalid content type for %s: %s (expected binary/octet-stream) (expected application/pdf)", finalURL, contentType)
		return false
	}

	var buf bytes.Buffer                     // Create a buffer to hold response data
	written, err := io.Copy(&buf, resp.Body) // Copy data into buffer
	if err != nil {
		log.Printf("Failed to read PDF data from %s: %v", finalURL, err)
		return false
	}
	if written == 0 { // Skip empty files
		log.Printf("Downloaded 0 bytes for %s; not creating file", finalURL)
		return false
	}

	out, err := os.Create(filePath) // Create output file
	if err != nil {
		log.Printf("Failed to create file for %s: %v", finalURL, err)
		return false
	}
	defer out.Close() // Ensure file is closed after writing

	if _, err := buf.WriteTo(out); err != nil { // Write buffer contents to file
		log.Printf("Failed to write PDF to file for %s: %v", finalURL, err)
		return false
	}

	log.Printf("Successfully downloaded %d bytes: %s → %s", written, finalURL, filePath) // Log success
	return true
}

// Performs HTTP GET request and returns response body as string
func getDataFromURL(uri string) string {
	log.Println("Scraping", uri)   // Log which URL is being scraped
	response, err := http.Get(uri) // Send GET request
	if err != nil {
		log.Println(err) // Log if request fails
	}

	body, err := io.ReadAll(response.Body) // Read the body of the response
	if err != nil {
		log.Println(err) // Log read error
	}

	err = response.Body.Close() // Close response body
	if err != nil {
		log.Println(err) // Log error during close
	}
	return string(body) // Return response body as string
}

// Append and write to file
func appendAndWriteToFile(path string, content string) {
	filePath, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	_, err = filePath.WriteString(content + "\n")
	if err != nil {
		log.Println(err)
	}
	err = filePath.Close()
	if err != nil {
		log.Println(err)
	}
}

// Read a file and return the contents
func readAFileAsString(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Println(err)
	}
	return string(content)
}

func main() {
	outputDir := "PDFs/" // Directory to store downloaded PDFs

	if !directoryExists(outputDir) { // Check if directory exists
		createDirectory(outputDir, 0o755) // Create directory with read-write-execute permissions
	}

	// The location to the local.
	localFile := "nclonline.html"
	// Check if the local file exists.
	if fileExists(localFile) {
		removeFile(localFile)
	}
	// The location to the remote url.
	remoteURL := []string{
		"https://www.nclonline.com/products/sds_alpha",
		"https://www.nclonline.com/products/view/15_COCONUT_OIL",
		"https://www.nclonline.com/products/view/24_7_",
		"https://www.nclonline.com/products/view/Afia_ALCOHOL_BASED",
		"https://www.nclonline.com/products/view/Afia_Alcohol_Free",
		"https://www.nclonline.com/products/view/Afia_Anti_Bacterial",
		"https://www.nclonline.com/products/view/Afia_Earth_Sense_Certified_Green_Foaming",
		"https://www.nclonline.com/products/view/Afia_Foaming_E2",
		"https://www.nclonline.com/products/view/Afia_Foaming_Hair_and_Body_Wash",
		"https://www.nclonline.com/products/view/Afia_Harvest_Melon",
		"https://www.nclonline.com/products/view/Afia_Hypoallergenic_Certified",
		"https://www.nclonline.com/products/view/Afia_Ocean_Mist",
		"https://www.nclonline.com/products/view/Afia_Spring_Blossom",
		"https://www.nclonline.com/products/view/ALL_IN_ONE_",
		"https://www.nclonline.com/products/view/ALL_OFF_",
		"https://www.nclonline.com/products/view/ASAP",
		"https://www.nclonline.com/products/view/ASTRO_CHEM_",
		"https://www.nclonline.com/products/view/AUTO_KLEEN_",
		"https://www.nclonline.com/products/view/AVISTAT_D_",
		"https://www.nclonline.com/products/view/BALANCE_",
		"https://www.nclonline.com/products/view/BARE_BONES_",
		"https://www.nclonline.com/products/view/BARE_BONES_LOW_ODOR",
		"https://www.nclonline.com/products/view/BATHROOM_PLUS_",
		"https://www.nclonline.com/products/view/BIG_PUNCH",
		"https://www.nclonline.com/products/view/BLUE_VELVET_",
		"https://www.nclonline.com/products/view/BOLT_",
		"https://www.nclonline.com/products/view/BRITE_EYES_",
		"https://www.nclonline.com/products/view/BULLSEYE_",
		"https://www.nclonline.com/products/view/BURST_PLUS_",
		"https://www.nclonline.com/products/view/C_ALL_",
		"https://www.nclonline.com/products/view/CHEM_EEZ_",
		"https://www.nclonline.com/products/view/CITRI_SCRUB_",
		"https://www.nclonline.com/products/view/CITROL",
		"https://www.nclonline.com/products/view/CITRUS_FLOWER_QUAT",
		"https://www.nclonline.com/products/view/CITRUS_KLEEN",
		"https://www.nclonline.com/products/view/CleanSMART_Foaming_Degreaser_Cleaner_SC",
		"https://www.nclonline.com/products/view/CleanSMART_Pot_Pan_Detergent_SC",
		"https://www.nclonline.com/products/view/CleanSMART_Sanitizer_1_512",
		"https://www.nclonline.com/products/view/COMBAT_",
		"https://www.nclonline.com/products/view/COMMAND_",
		"https://www.nclonline.com/products/view/CONKLEEN_204_",
		"https://www.nclonline.com/products/view/CORRAL_",
		"https://www.nclonline.com/products/view/CREAM_COAT_",
		"https://www.nclonline.com/products/view/CYCLONE_",
		"https://www.nclonline.com/products/view/DECADE_",
		"https://www.nclonline.com/products/view/DEO_PINE_",
		"https://www.nclonline.com/products/view/DESCUM",
		"https://www.nclonline.com/products/view/DUAL_BLEND_1",
		"https://www.nclonline.com/products/view/DUAL_BLEND_10",
		"https://www.nclonline.com/products/view/DUAL_BLEND_11",
		"https://www.nclonline.com/products/view/DUAL_BLEND_17",
		"https://www.nclonline.com/products/view/DUAL_BLEND_19",
		"https://www.nclonline.com/products/view/DUAL_BLEND_2",
		"https://www.nclonline.com/products/view/DUAL_BLEND_20",
		"https://www.nclonline.com/products/view/DUAL_BLEND_21",
		"https://www.nclonline.com/products/view/DUAL_BLEND_22",
		"https://www.nclonline.com/products/view/DUAL_BLEND_23",
		"https://www.nclonline.com/products/view/DUAL_BLEND_24",
		"https://www.nclonline.com/products/view/DUAL_BLEND_25",
		"https://www.nclonline.com/products/view/DUAL_BLEND_26",
		"https://www.nclonline.com/products/view/DUAL_BLEND_3",
		"https://www.nclonline.com/products/view/DUAL_BLEND_4",
		"https://www.nclonline.com/products/view/DUAL_BLEND_5",
		"https://www.nclonline.com/products/view/DUAL_BLEND_6",
		"https://www.nclonline.com/products/view/DUAL_BLEND_7",
		"https://www.nclonline.com/products/view/DUAL_BLEND_8",
		"https://www.nclonline.com/products/view/DUAL_BLEND_9",
		"https://www.nclonline.com/products/view/DURA_GLOSS_",
		"https://www.nclonline.com/products/view/EARTH_SENSE_ASPIRE_",
		"https://www.nclonline.com/products/view/EARTH_SENSE_Certified_Foaming_Hand_Cleaner",
		"https://www.nclonline.com/products/view/EARTH_SENSE_Certified_Liquid_Hand_Cleaner",
		"https://www.nclonline.com/products/view/EARTH_SENSE_Degreaser_Cleaner",
		"https://www.nclonline.com/products/view/EARTH_SENSE_EVERGREEN_FINISH",
		"https://www.nclonline.com/products/view/EARTH_SENSE_Extra_Heavy_Duty_RTU",
		"https://www.nclonline.com/products/view/EARTH_SENSE_Foam_Safe",
		"https://www.nclonline.com/products/view/EARTH_SENSE_GREEN_IMPACT_",
		"https://www.nclonline.com/products/view/EARTH_SENSE_HD_WASHROOM_CLEANER",
		"https://www.nclonline.com/products/view/EARTH_SENSE_Multi_Purpose_Neutral_Cleaner",
		"https://www.nclonline.com/products/view/EARTH_SENSE_Multi_Surface_Concentrate_with_H2O2",
		"https://www.nclonline.com/products/view/EARTH_SENSE_NEUTRAL_FLOOR_CLEANER",
		"https://www.nclonline.com/products/view/EARTH_SENSE_RTU_GLASS_HARD_SURFACE_CLEANER",
		"https://www.nclonline.com/products/view/EASY_DAB_",
		"https://www.nclonline.com/products/view/ECO_SOLV",
		"https://www.nclonline.com/products/view/EDGE_PLUS_",
		"https://www.nclonline.com/products/view/ENDURE_",
		"https://www.nclonline.com/products/view/ENHANCE_",
		"https://www.nclonline.com/products/view/ENSEEL_",
		"https://www.nclonline.com/products/view/ES_Neutral_Disinfectant_Detergent",
		"https://www.nclonline.com/products/view/ETERNITY_",
		"https://www.nclonline.com/products/view/ETERNITY_Aerosol_",
		"https://www.nclonline.com/products/view/EXPOSE_",
		"https://www.nclonline.com/products/view/EXTREME_PLUS_",
		"https://www.nclonline.com/products/view/FLEXI_CLEAN",
		"https://www.nclonline.com/products/view/FLEXI_SHEEN_",
		"https://www.nclonline.com/products/view/FOAM_SAFE_OCEAN_MIST",
		"https://www.nclonline.com/products/view/FOAM_BREAK_",
		"https://www.nclonline.com/products/view/FORTRESS",
		"https://www.nclonline.com/products/view/FRESH_START_",
		"https://www.nclonline.com/products/view/GLIMMER_",
		"https://www.nclonline.com/products/view/GOLDEN_POT_PAN",
		"https://www.nclonline.com/products/view/GREEN_EMERALD",
		"https://www.nclonline.com/products/view/HOMBRE_",
		"https://www.nclonline.com/products/view/HURRAH_CAR_WASH",
		"https://www.nclonline.com/products/view/HURRICANE_",
		"https://www.nclonline.com/products/view/IMAGE_",
		"https://www.nclonline.com/products/view/IMPRESSIONS_",
		"https://www.nclonline.com/products/view/INCREDILOSO_",
		"https://www.nclonline.com/products/view/INCREDILOSO_Lavender",
		"https://www.nclonline.com/products/view/INVINCIBLE_",
		"https://www.nclonline.com/products/view/KITCHEN_MATE",
		"https://www.nclonline.com/products/view/KLEER_BRITE_",
		"https://www.nclonline.com/products/view/LAVENDER_QUAT",
		"https://www.nclonline.com/products/view/LEMON_QUAT",
		"https://www.nclonline.com/products/view/LUSTER",
		"https://www.nclonline.com/products/view/LVT_CLEAN",
		"https://www.nclonline.com/products/view/LVT_PROTECT",
		"https://www.nclonline.com/products/view/MAGIC_BREEZE_Herbal",
		"https://www.nclonline.com/products/view/MAGIC_BREEZE_Lavender",
		"https://www.nclonline.com/products/view/MAIN_SQUEEZE_CLEANER",
		"https://www.nclonline.com/products/view/MAIN_SQUEEZE_DEGREASER",
		"https://www.nclonline.com/products/view/MAIN_SQUEEZE_GLASS",
		"https://www.nclonline.com/products/view/MAIN_SQUEEZE_Lavender_256",
		"https://www.nclonline.com/products/view/MARVEL",
		"https://www.nclonline.com/products/view/MATTE",
		"https://www.nclonline.com/products/view/MICRO_CHEM_PLUS_",
		"https://www.nclonline.com/products/view/MINT_QUAT",
		"https://www.nclonline.com/products/view/MIRAGE",
		"https://www.nclonline.com/products/view/MOLD_AWAY_",
		"https://www.nclonline.com/products/view/MRP_",
		"https://www.nclonline.com/products/view/MULTI_STAT_",
		"https://www.nclonline.com/products/view/NATURAL_MIRACLE_",
		"https://www.nclonline.com/products/view/NATURE_S_FORCE",
		"https://www.nclonline.com/products/view/NATURE_S_POWER",
		"https://www.nclonline.com/products/view/NATURE_S_SOLUTION_",
		"https://www.nclonline.com/products/view/NCL_2_",
		"https://www.nclonline.com/products/view/NCLwipes_Lemon_Fresh",
		"https://www.nclonline.com/products/view/NCLwipes_Waterfall_Fresh",
		"https://www.nclonline.com/products/view/NEUTRA_CIDE_256",
		"https://www.nclonline.com/products/view/NEUTRAL_Q_",
		"https://www.nclonline.com/products/view/NEXT_CENTURY_",
		"https://www.nclonline.com/products/view/NEXT_STEP_",
		"https://www.nclonline.com/products/view/NO_ZAP_STATIC_DISSIPATIVE_FLOOR_COATING",
		"https://www.nclonline.com/products/view/NU_HIDE_",
		"https://www.nclonline.com/products/view/NU_LOOK",
		"https://www.nclonline.com/products/view/ONE_COAT_25_",
		"https://www.nclonline.com/products/view/ONE_STEP_",
		"https://www.nclonline.com/products/view/ONE_",
		"https://www.nclonline.com/products/view/PATINA_",
		"https://www.nclonline.com/products/view/PERFECTION_",
		"https://www.nclonline.com/products/view/pH_ENOMENAL_",
		"https://www.nclonline.com/products/view/PICTURE_PERFECT_",
		"https://www.nclonline.com/products/view/PINE_QUAT_PLUS_",
		"https://www.nclonline.com/products/view/PINK_LOTION",
		"https://www.nclonline.com/products/view/PINK_N_CREAMY",
		"https://www.nclonline.com/products/view/PINK_SUDS",
		"https://www.nclonline.com/products/view/PIZZAZZ_",
		"https://www.nclonline.com/products/view/POOFF_",
		"https://www.nclonline.com/products/view/POP_SHINE_",
		"https://www.nclonline.com/products/view/POP_SHINE_RTU",
		"https://www.nclonline.com/products/view/PRO_SEEL_",
		"https://www.nclonline.com/products/view/ProLEX_CDL_520",
		"https://www.nclonline.com/products/view/ProLEX_HTR_260",
		"https://www.nclonline.com/products/view/ProLEX_LTD_220",
		"https://www.nclonline.com/products/view/ProLEX_LTR_250",
		"https://www.nclonline.com/products/view/QWIK_SCRUB_",
		"https://www.nclonline.com/products/view/RELY",
		"https://www.nclonline.com/products/view/RINSE_AWAY_PLUS_",
		"https://www.nclonline.com/products/view/ROAD_AWAY",
		"https://www.nclonline.com/products/view/ROCK_HARD_",
		"https://www.nclonline.com/products/view/RUFF_N_READY",
		"https://www.nclonline.com/products/view/SANIQUAT",
		"https://www.nclonline.com/products/view/SEA_BRITE_",
		"https://www.nclonline.com/products/view/SHA_ZYME_",
		"https://www.nclonline.com/products/view/SHA_ZYME_DRC",
		"https://www.nclonline.com/products/view/SHA_ZYME_RTU",
		"https://www.nclonline.com/products/view/SHIELD",
		"https://www.nclonline.com/products/view/SOFT_N_CREAMY",
		"https://www.nclonline.com/products/view/SPIT_SHINE_",
		"https://www.nclonline.com/products/view/SPRAY_KLEEN_PLUS_",
		"https://www.nclonline.com/products/view/SPRITZ_",
		"https://www.nclonline.com/products/view/STAMINA_",
		"https://www.nclonline.com/products/view/STONE_BEAUTY_",
		"https://www.nclonline.com/products/view/STONE_KLEEN_",
		"https://www.nclonline.com/products/view/SUN_SPRAY",
		"https://www.nclonline.com/products/view/SUPER_CHERRY",
		"https://www.nclonline.com/products/view/SUPER_NAC_",
		"https://www.nclonline.com/products/view/SUPER_PURGE",
		"https://www.nclonline.com/products/view/SUPER_SONIC_",
		"https://www.nclonline.com/products/view/SURFACE_BARRIER_",
		"https://www.nclonline.com/products/view/SURFACE_PREP_",
		"https://www.nclonline.com/products/view/SURGE_",
		"https://www.nclonline.com/products/view/TANNIN_OUT_",
		"https://www.nclonline.com/products/view/TOTAL_",
		"https://www.nclonline.com/products/view/TRIGGER_",
		"https://www.nclonline.com/products/view/TWISTER_",
		"https://www.nclonline.com/products/view/ULTRAMAX_",
		"https://www.nclonline.com/products/view/UPPER_HAND_",
		"https://www.nclonline.com/products/view/VIGOR_",
		"https://www.nclonline.com/products/view/VISIONS_",
		"https://www.nclonline.com/products/view/VIVID_",
		"https://www.nclonline.com/products/view/WASH_BRITE_",
		"https://www.nclonline.com/products/view/WHITE_PEARL",
		"https://www.nclonline.com/products/view/WITHSTAND_",
		"https://www.nclonline.com/products/view/WORLD_CLASS_",
		"https://www.nclonline.com/products/view/WRANGLER_",
		"https://www.nclonline.com/products/view/ZooooM_",
		"https://www.nclonline.com/products/flyer_alpha.php",
		"https://www.nclonline.com/products/view/Afia_Drip_Tray",
		"https://www.nclonline.com/products/view/Afia_Floor_Dispenser_Stand_White",
		"https://www.nclonline.com/products/view/Afia_Manual_Dispenser",
		"https://www.nclonline.com/products/view/Afia_Touch_Free",
		"https://www.nclonline.com/products/view/CleanSMART_Foam_Dispensing_Unit",
		"https://www.nclonline.com/products/view/CleanSMART_Sink_Dispensing_Unit",
		"https://www.nclonline.com/products/view/DUAL_BLEND_Jr_",
		"https://www.nclonline.com/products/view/DUAL_BLEND_PORTABLE",
		"https://www.nclonline.com/products/view/DUAL_BLEND_PORTABLE_KIT",
		"https://www.nclonline.com/products/view/DUAL_BLEND_WALL",
		"https://www.nclonline.com/products/view/ECONO_DIAMONDS",
		"https://www.nclonline.com/products/view/FOAM_MAGIC",
		"https://www.nclonline.com/products/view/GRANITE_MASTER_",
		"https://www.nclonline.com/products/view/HANDI_RACK_Round_Gallon_",
		"https://www.nclonline.com/products/view/INDUSTRIAL_HAND_SOAP_DISPENSER",
		"https://www.nclonline.com/products/view/LUMINAIRE_",
		"https://www.nclonline.com/products/view/MECHANICS_SELECT_HAND_CARE_PUMP",
		"https://www.nclonline.com/products/view/NAT_SPEED_",
		"https://www.nclonline.com/products/view/NAT_SPLASH_GUARD",
		"https://www.nclonline.com/products/view/NAT_STONE_Pad_Driver",
		"https://www.nclonline.com/products/view/Pak_SMART_Cap",
		"https://www.nclonline.com/products/view/PRO_SERIES_STONE_BLAZER_",
		"https://www.nclonline.com/products/view/Refillable_Foaming_Hand_Cleaner_Dispener_Cartridge",
		"https://www.nclonline.com/products/view/RSC_Foaming_Nozzle",
		"https://www.nclonline.com/products/view/STONE_BLAZER_",
		"https://www.nclonline.com/products/view/UNI_POWER_",
		"https://www.nclonline.com/products/view/WET_CONCRETE_DIAMONDS",
	}
	// Loop over the urls and save content to file.
	for _, url := range remoteURL {
		// Call fetchPage to download the content of that page
		pageContent := getDataFromURL(url)
		// Append it and save it to the file.
		appendAndWriteToFile(localFile, pageContent)
	}
	// Read the file content
	fileContent := readAFileAsString(localFile)
	// Extract the URLs from the given content.
	extractedPDFURLs := extractPDFUrls(fileContent)
	// Remove duplicates from the slice.
	extractedPDFURLs = removeDuplicatesFromSlice(extractedPDFURLs)
	// Loop through all extracted PDF URLs
	for _, urls := range extractedPDFURLs {
		if !hasDomain(urls) {
			urls = "https://www.nclonline.com" + urls

		}
		if isUrlValid(urls) { // Check if the final URL is valid
			downloadPDF(urls, outputDir) // Download the PDF
		}
	}
}
