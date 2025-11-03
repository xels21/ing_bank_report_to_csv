package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dslipak/pdf"
)

func main() {
	inDir := flag.String("in", "./in", "Input directory to search for PDF files")
	outDir := flag.String("out", "./out", "Output directory to save CSV files")
	flag.Parse()

	fmt.Printf("Input directory: %s\n", *inDir)
	fmt.Printf("Output directory: %s\n", *outDir)

	// Create output directory if it doesn't exist
	if _, err := os.Stat(*outDir); os.IsNotExist(err) {
		os.MkdirAll(*outDir, os.ModePerm)
	}

	err := filepath.Walk(*inDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".pdf") {
			fmt.Printf("Processing file: %s\n", path)
			processPdf(path, *outDir)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking the path %q: %v\n", *inDir, err)
	}
}

func processPdf(pdfPath string, outDir string) {
	r, err := pdf.Open(pdfPath)
	if err != nil {
		fmt.Printf("Error opening PDF file %s: %v\n", pdfPath, err)
		return
	}

	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		fmt.Printf("Error extracting text from %s: %v\n", pdfPath, err)
		return
	}
	if _, err := buf.ReadFrom(b); err != nil {
		fmt.Printf("Error reading from buffer for %s: %v\n", pdfPath, err)
		return
	}

	records := pdfStringToCsv(buf.String())

	if len(records) > 0 {
		csvFileName := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath)) + ".csv"
		csvFilePath := filepath.Join(outDir, csvFileName)

		csvFile, err := os.Create(csvFilePath)
		if err != nil {
			fmt.Printf("Error creating CSV file %s: %v\n", csvFilePath, err)
			return
		}
		defer csvFile.Close()

		writer := csv.NewWriter(csvFile)
		writer.Comma = ';'
		defer writer.Flush()

		writer.Write([]string{"data", "description", "value"})
		writer.WriteAll(records)

		fmt.Printf("Successfully converted %s to %s\n", pdfPath, csvFilePath)
	} else {
		fmt.Printf("No records found in %s\n", pdfPath)
	}
}
func pdfStringToCsv(raw string) [][]string {
	// Header cut only if 'Valuta' appears BEFORE first date (to retain early transactions)
	dateRePre := regexp.MustCompile(`(\d{2}\.\d{2}\.\d{4})`)
	firstDateLoc := dateRePre.FindStringIndex(raw)
	valutaIdx := strings.Index(raw, "Valuta")
	if valutaIdx != -1 && (firstDateLoc == nil || valutaIdx < firstDateLoc[0]) {
		raw = raw[valutaIdx+len("Valuta"):]
	}
	// Removed aggressive footer truncation to allow detection of later dates beyond legal blocks.

	dateRe := regexp.MustCompile(`(\d{2}\.\d{2}\.\d{4})`)
	amountRe := regexp.MustCompile(`-?[0-9]{1,3}(?:\.[0-9]{3})*,[0-9]{2}$`)
	noiseKeywords := []string{"Datum", "Auszugsnummer", "Buchung", "Valuta", "IBAN", "BIC", "Seite", "ING-DiBa AG", "Herrn"}
	summaryMarkers := []string{"Neuer Saldo", "Alter Saldo", "Kunden-Information", "Kontoüberziehung", "Vorliegender Freistellungsauftrag", "Bitte beachten"}
	trimSummary := func(s string) (string, bool) {
		min := -1
		for _, m := range summaryMarkers {
			if idx := strings.Index(s, m); idx >= 0 {
				if min == -1 || idx < min {
					min = idx
				}
			}
		}
		if min >= 0 {
			return strings.TrimSpace(s[:min]), true
		}
		return s, false
	}
	cutNoiseTail := func(s string) string {
		minIdx := -1
		for _, kw := range noiseKeywords {
			if idx := strings.Index(s, kw); idx >= 0 {
				if minIdx == -1 || idx < minIdx {
					minIdx = idx
				}
			}
		}
		// detect first non-ASCII rune cluster (>=3 consecutive) as noise start
		runes := []rune(s)
		clusterStart := -1
		clusterLen := 0
		for i, r := range runes {
			if r < 32 || r > 126 {
				if clusterStart == -1 {
					clusterStart = i
				}
				clusterLen++
				if clusterLen >= 3 {
					break
				}
			} else {
				clusterStart = -1
				clusterLen = 0
			}
		}
		if clusterLen >= 3 {
			pos := 0
			for j := 0; j < clusterStart; j++ {
				pos += len(string(runes[j]))
			}
			if minIdx == -1 || pos < minIdx {
				minIdx = pos
			}
		}
		if minIdx >= 0 {
			return strings.TrimSpace(s[:minIdx])
		}
		// fallback strip non-ASCII entirely
		filtered := strings.Map(func(r rune) rune {
			if r < 32 || r > 126 {
				return -1
			}
			return r
		}, s)
		return strings.TrimSpace(filtered)
	}

	idxs := dateRe.FindAllStringIndex(raw, -1)
	if len(idxs) == 0 {
		return nil
	}

	type seg struct{ date, body string }
	segs := make([]seg, 0, len(idxs))
	for i, p := range idxs {
		date := raw[p[0]:p[1]]
		start := p[1]
		end := len(raw)
		if i+1 < len(idxs) {
			end = idxs[i+1][0]
		}
		body := strings.TrimSpace(raw[start:end])
		segs = append(segs, seg{date: date, body: body})
	}

	var results [][]string
	consumedUntil := 0
	for i := 0; i < len(segs); i++ {
		current := segs[i]
		trim := strings.TrimSpace(current.body)
		// Case A: segment already has amount -> build record and optionally absorb following same-date no-amount segments
		if amountRe.MatchString(trim) {
			amtIdx := amountRe.FindStringIndex(trim)
			before := strings.TrimSpace(trim[:amtIdx[0]])
			amt := trim[amtIdx[0]:]
			descParts := []string{}
			if before != "" {
				descParts = append(descParts, before)
			}
			// If there is a second same-date segment immediately preceding additional descriptive tokens (like reference numbers)
			j := i + 1
			for j < len(segs) {
				if segs[j].date != current.date {
					break
				}
				nextRaw := strings.TrimSpace(segs[j].body)
				if nextRaw == "" {
					j++
					continue
				}
				trimmed := cutNoiseTail(nextRaw)
				trimmed, hadSummary := trimSummary(trimmed)
				if hadSummary {
					if trimmed != "" {
						descParts = append(descParts, trimmed)
					}
					break
				}
				if trimmed == "" {
					j++
					continue
				}
				// allow merging of following segment even if amount already captured; if another amount found, stop
				if amountRe.MatchString(trimmed) {
					break
				}
				descParts = append(descParts, trimmed)
				j++
			}
			results = append(results, []string{current.date, strings.Join(descParts, " - "), amt})
			// Track consumed position to allow second pass on remaining raw. Use end of segment instead of end of date token.
			consumedUntil = idxs[i][1]
			// advance consumedUntil to end of merged lookahead segments
			if j-1 < len(idxs) {
				consumedUntil = idxs[j-1][1]
			}
			for _, segAppended := range descParts {
				_ = segAppended // placeholder to indicate description processed
			}
			i = j - 1
			continue
		}
		// Case B: no amount yet -> accumulate until amount appears for this date
		descAcc := []string{}
		if trim != "" {
			descAcc = append(descAcc, trim)
		}
		j := i + 1
		amount := ""
		for j < len(segs) {
			if segs[j].date != current.date { // different date before amount -> abandon
				break
			}
			nRaw := strings.TrimSpace(segs[j].body)
			if nRaw == "" {
				j++
				continue
			}
			nTrim := cutNoiseTail(nRaw)
			nTrim, hadSummary := trimSummary(nTrim)
			if nTrim == "" {
				j++
				continue
			}
			if hadSummary {
				// summary appears before amount -> abandon this date (no amount yet) and stop accumulation
				break
			}
			if amountRe.MatchString(nTrim) {
				amtIdx := amountRe.FindStringIndex(nTrim)
				before := strings.TrimSpace(nTrim[:amtIdx[0]])
				if before != "" {
					descAcc = append(descAcc, before)
				}
				amount = nTrim[amtIdx[0]:]
				// absorb trailing same-date no-amount after amount
				k := j + 1
				for k < len(segs) {
					if segs[k].date != current.date {
						break
					}
					postRaw := strings.TrimSpace(segs[k].body)
					if postRaw == "" {
						k++
						continue
					}
					postTrim := cutNoiseTail(postRaw)
					postTrim, hadSummary := trimSummary(postTrim)
					if postTrim == "" {
						k++
						continue
					}
					if hadSummary { // include trimmed before summary then stop
						descAcc = append(descAcc, postTrim)
						break
					}
					if amountRe.MatchString(postTrim) {
						break
					}
					descAcc = append(descAcc, postTrim)
					k++
				}
				i = k - 1
				break
			} else {
				descAcc = append(descAcc, nTrim)
			}
			j++
		}
		if amount != "" {
			results = append(results, []string{current.date, strings.Join(descAcc, " - "), amount})
			consumedUntil = idxs[i][1]
		} else {
			// no amount encountered: do not emit
		}
	}
	// Second pass: parse remaining tail (raw after last consumedUntil) for additional date groups not captured due to earlier truncation logic
	if consumedUntil > 0 && consumedUntil < len(raw) {
		remaining := raw[consumedUntil:]
		idxs2 := dateRe.FindAllStringIndex(remaining, -1)
		for k := 0; k < len(idxs2); k++ {
			startDate := remaining[idxs2[k][0]:idxs2[k][1]]
			startContentStart := idxs2[k][1]
			end := len(remaining)
			if k+1 < len(idxs2) {
				end = idxs2[k+1][0]
			}
			content := strings.TrimSpace(remaining[startContentStart:end])
			if content == "" {
				continue
			}
			// Remove trailing noise lines from content
			for _, kw := range noiseKeywords {
				if pos := strings.Index(content, kw); pos >= 0 {
					content = strings.TrimSpace(content[:pos])
				}
			}
			for _, sm := range summaryMarkers {
				if pos := strings.Index(content, sm); pos >= 0 {
					content = strings.TrimSpace(content[:pos])
					break
				}
			}
			if amountRe.MatchString(content) {
				amtIdx := amountRe.FindStringIndex(content)
				before := strings.TrimSpace(content[:amtIdx[0]])
				amt := content[amtIdx[0]:]
				desc := before
				// absorb same-date following segments without amount
				m := k + 1
				parts := []string{}
				if desc != "" {
					parts = append(parts, desc)
				}
				for m < len(idxs2) {
					nextDate := remaining[idxs2[m][0]:idxs2[m][1]]
					if nextDate != startDate {
						break
					}
					nStart := idxs2[m][1]
					nEnd := len(remaining)
					if m+1 < len(idxs2) {
						nEnd = idxs2[m+1][0]
					}
					nContent := strings.TrimSpace(remaining[nStart:nEnd])
					if nContent == "" || amountRe.MatchString(nContent) {
						break
					}
					for _, kw := range noiseKeywords {
						if pos := strings.Index(nContent, kw); pos >= 0 {
							nContent = strings.TrimSpace(nContent[:pos])
						}
					}
					parts = append(parts, nContent)
					m++
				}
				results = append(results, []string{startDate, strings.Join(parts, " - "), amt})
				k = m - 1
			} else {
				// accumulate until amount in remaining tail similarly
			}
		}
	}
	return results
}
