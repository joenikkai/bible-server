package main

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed Bible.txt
var BibleFS embed.FS

var BookMap = []string{
	"Book 01 Genesis", "Book 02 Exodus", "Book 03 Leviticus", "Book 04 Numbers",
	"Book 05 Deuteronomy", "Book 06 Joshua", "Book 07 Judges", "Book 08 Ruth",
	"Book 09 1 Samuel", "Book 10 2 Samuel", "Book 11 1 Kings", "Book 12 2 Kings",
	"Book 13 1 Chronicles", "Book 14 2 Chronicles", "Book 15 Ezra", "Book 16 Nehemiah",
	"Book 17 Esther", "Book 18 Job", "Book 19 Psalms", "Book 20 Proverbs",
	"Book 21 Ecclesiastes", "Book 22 Song of Solomon", "Book 23 Isaiah",
	"Book 24 Jeremiah", "Book 25 Lamentations", "Book 26 Ezekiel", "Book 27 Daniel",
	"Book 28 Hosea", "Book 29 Joel", "Book 30 Amos", "Book 31 Obadiah",
	"Book 32 Jonah", "Book 33 Micah", "Book 34 Nahum", "Book 35 Habakkuk",
	"Book 36 Zephaniah", "Book 37 Haggai", "Book 38 Zechariah", "Book 39 Malachi",
	"Book 40 Matthew", "Book 41 Mark", "Book 42 Luke", "Book 43 John",
	"Book 44 Acts", "Book 45 Romans", "Book 46 1 Corinthians", "Book 47 2 Corinthians",
	"Book 48 Galatians", "Book 49 Ephesians", "Book 50 Philippians",
	"Book 51 Colossians", "Book 52 1 Thessalonians", "Book 53 2 Thessalonians",
	"Book 54 1 Timothy", "Book 55 2 Timothy", "Book 56 Titus", "Book 57 Philemon",
	"Book 58 Hebrews", "Book 59 James", "Book 60 1 Peter", "Book 61 2 Peter",
	"Book 62 1 John", "Book 63 2 John", "Book 64 3 John", "Book 65 Jude",
	"Book 66 Revelation",
}

type Verse struct {
	Book      string `json:"book"`
	Chapter   int    `json:"chapter"`
	Verse     int    `json:"verse"`
	Reference string `json:"reference"`
	Text      string `json:"text"`
}

var (
	allVerses []Verse
	index     map[string]map[int]map[int]int // book -> chapter -> verse -> index in allVerses
	bookOrder []string                       // insertion order from the file
)

func main() {
	fmt.Println("Starting bible server")
	if err := loadBible(); err != nil {
		log.Fatalf("failed to load Bible.txt: %v", err)
	}
	log.Printf("Loaded %d verses across %d books", len(allVerses), len(bookOrder))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleInit)                       // exact "/"
	mux.HandleFunc("GET /random", handleRandom)                  // GET /random?count=N&book=X
	mux.HandleFunc("GET /{book}", handleBook)                    // GET /Genesis
	mux.HandleFunc("GET /{book}/{chapter}", handleChapter)       // GET /Genesis/1
	mux.HandleFunc("GET /{book}/{chapter}/{verse}", handleVerse) // GET /Genesis/1/1-10

	port := ":7000"
	fmt.Println("server listening on http://localhost" + port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatal(err)
	}
}

// ---------------- Loader ----------------

func loadBible() error {
	f, err := BibleFS.Open("Bible.txt")
	if err != nil {
		return err
	}
	defer f.Close()

	index = make(map[string]map[int]map[int]int)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // up to 1 MB per line

	var (
		currentBook string
		// "Book NN Name ...rest"
		bookRe = regexp.MustCompile(`^(Book\s+\d+\s+.+?)\s*$`)
		// "Book NN Name 1:1 text"
		bookVerseRe = regexp.MustCompile(`^(Book\s+\d+\s+.+?)\s+(\d+):(\d+)\s*(.*)$`)
		// "1:1 text"
		verseRe = regexp.MustCompile(`^(\d+):(\d+)\s*(.*)$`)
	)

	for scanner.Scan() {
		line := strings.TrimPrefix(strings.TrimRight(scanner.Text(), "\r"), "\ufeff")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// "Book NN Name C:V text" (inline)
		if m := bookVerseRe.FindStringSubmatch(trimmed); m != nil {
			book := strings.TrimSpace(m[1])
			ch, _ := strconv.Atoi(m[2])
			vs, _ := strconv.Atoi(m[3])
			addVerse(book, ch, vs, cleanText(m[4]))
			currentBook = book
			continue
		}

		// "Book NN Name" (header)
		if m := bookRe.FindStringSubmatch(trimmed); m != nil {
			currentBook = strings.TrimSpace(m[1])
			if _, ok := index[currentBook]; !ok {
				index[currentBook] = make(map[int]map[int]int)
				bookOrder = append(bookOrder, currentBook)
			}
			continue
		}

		// "C:V text"
		if m := verseRe.FindStringSubmatch(trimmed); m != nil && currentBook != "" {
			ch, _ := strconv.Atoi(m[1])
			vs, _ := strconv.Atoi(m[2])
			addVerse(currentBook, ch, vs, cleanText(m[3]))
			continue
		}

		// Continuation of the previous verse
		if len(allVerses) > 0 {
			allVerses[len(allVerses)-1].Text += " " + trimmed
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(allVerses) == 0 {
		return fmt.Errorf("no verses parsed; check Bible.txt format")
	}
	return nil
}

func cleanText(s string) string {
	return strings.TrimLeft(strings.TrimSpace(s), ":.-— \t")
}

func addVerse(book string, ch, vs int, text string) {
	if _, ok := index[book]; !ok {
		index[book] = make(map[int]map[int]int)
		bookOrder = append(bookOrder, book)
	}
	if _, ok := index[book][ch]; !ok {
		index[book][ch] = make(map[int]int)
	}
	idx := len(allVerses)
	allVerses = append(allVerses, Verse{
		Book:      book,
		Chapter:   ch,
		Verse:     vs,
		Reference: fmt.Sprintf("%s %d:%d", shortBook(book), ch, vs),
		Text:      strings.TrimSpace(text),
	})
	index[book][ch][vs] = idx
}

func shortBook(full string) string {
	if parts := strings.SplitN(full, " ", 3); len(parts) == 3 {
		return parts[2]
	}
	return full
}

// ---------------- Handlers ----------------

// GET /
func handleInit(w http.ResponseWriter, r *http.Request) {
	type BookInfo struct {
		Name     string `json:"name"`
		Short    string `json:"short"`
		Chapters int    `json:"chapters"`
		Verses   int    `json:"verses"`
	}
	out := struct {
		TotalBooks  int        `json:"total_books"`
		TotalVerses int        `json:"total_verses"`
		Books       []BookInfo `json:"books"`
	}{TotalBooks: len(bookOrder), TotalVerses: len(allVerses)}

	for _, b := range bookOrder {
		v := 0
		for _, ch := range index[b] {
			v += len(ch)
		}
		out.Books = append(out.Books, BookInfo{
			Name:     b,
			Short:    shortBook(b),
			Chapters: len(index[b]),
			Verses:   v,
		})
	}
	writeJSON(w, out)
}

// GET /random?count=N&book=Genesis
func handleRandom(w http.ResponseWriter, r *http.Request) {
	if len(allVerses) == 0 {
		http.Error(w, "no verses loaded", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()

	pool := allVerses
	if b := q.Get("book"); b != "" {
		book := resolveBook(b)
		if book == "" {
			http.Error(w, "book not found: "+b, http.StatusNotFound)
			return
		}
		pool = make([]Verse, 0, 300)
		for _, v := range allVerses {
			if v.Book == book {
				pool = append(pool, v)
			}
		}
	}

	count := 1
	if c := q.Get("count"); c != "" {
		if n, err := strconv.Atoi(c); err == nil && n > 0 && n <= 100 {
			count = n
		}
	}

	result := make([]Verse, count)
	for i := range result {
		result[i] = pool[rand.Intn(len(pool))]
	}
	writeJSON(w, result)
}

// GET /Genesis
func handleBook(w http.ResponseWriter, r *http.Request) {
	book := resolveBook(r.PathValue("book"))
	if book == "" {
		http.Error(w, "book not found", http.StatusNotFound)
		return
	}
	type ChInfo struct {
		Chapter int `json:"chapter"`
		Verses  int `json:"verses"`
	}
	chs := sortedChapters(index[book])
	out := make([]ChInfo, len(chs))
	for i, c := range chs {
		out[i] = ChInfo{Chapter: c, Verses: len(index[book][c])}
	}
	writeJSON(w, out)
}

// GET /Genesis/1
func handleChapter(w http.ResponseWriter, r *http.Request) {
	book := resolveBook(r.PathValue("book"))
	if book == "" {
		http.Error(w, "book not found", http.StatusNotFound)
		return
	}
	chStart, chEnd, err := parseChapterRange(r.PathValue("chapter"))
	if err != nil {
		http.Error(w, "invalid chapter: "+err.Error(), http.StatusBadRequest)
		return
	}
	result := collectVerses(book, chStart, chEnd, 1, 1<<30, 0)
	respondWithVerses(w, result)
}

// GET /Genesis/1/1   |   /Genesis/1/1-10   |   /Genesis/1/1-2:5   |   /Genesis/1-3/1-10
func handleVerse(w http.ResponseWriter, r *http.Request) {
	book := resolveBook(r.PathValue("book"))
	if book == "" {
		http.Error(w, "book not found", http.StatusNotFound)
		return
	}
	chStart, chEnd, err := parseChapterRange(r.PathValue("chapter"))
	if err != nil {
		http.Error(w, "invalid chapter: "+err.Error(), http.StatusBadRequest)
		return
	}
	vStart, vEnd, vEndCh, err := parseVerseRange(r.PathValue("verse"))
	if err != nil {
		http.Error(w, "invalid verse: "+err.Error(), http.StatusBadRequest)
		return
	}
	if chStart != chEnd && vEndCh > 0 {
		http.Error(w, "cannot combine chapter range with cross-chapter verse range", http.StatusBadRequest)
		return
	}
	result := collectVerses(book, chStart, chEnd, vStart, vEnd, vEndCh)
	respondWithVerses(w, result)
}

// ---------------- Parsers / helpers ----------------

func resolveBook(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	for _, b := range bookOrder {
		if strings.EqualFold(b, name) {
			return b
		}
		if parts := strings.SplitN(b, " ", 3); len(parts) == 3 {
			if strings.EqualFold(parts[2], name) { // "Genesis", "1 Samuel"
				return b
			}
			if parts[1] == name { // "01"
				return b
			}
			if strings.EqualFold(parts[1]+" "+parts[2], name) { // "01 Genesis"
				return b
			}
		}
	}
	return ""
}

func parseChapterRange(s string) (int, int, error) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "-"); i >= 0 {
		a, err := strconv.Atoi(strings.TrimSpace(s[:i]))
		if err != nil {
			return 0, 0, err
		}
		b, err := strconv.Atoi(strings.TrimSpace(s[i+1:]))
		if err != nil {
			return 0, 0, err
		}
		if b < a {
			return 0, 0, fmt.Errorf("chapter end < start")
		}
		return a, b, nil
	}
	n, err := strconv.Atoi(s)
	return n, n, err
}

// Parses verse segment: "5", "5-10", "5-2:3".
// Returns (startVerse, endVerse, endChapter) where endChapter==0 means "same chapter".
func parseVerseRange(s string) (int, int, int, error) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "-"); i >= 0 {
		a, err := strconv.Atoi(strings.TrimSpace(s[:i]))
		if err != nil {
			return 0, 0, 0, err
		}
		tail := strings.TrimSpace(s[i+1:])
		if j := strings.Index(tail, ":"); j >= 0 {
			ec, err := strconv.Atoi(strings.TrimSpace(tail[:j]))
			if err != nil {
				return 0, 0, 0, err
			}
			ev, err := strconv.Atoi(strings.TrimSpace(tail[j+1:]))
			if err != nil {
				return 0, 0, 0, err
			}
			return a, ev, ec, nil
		}
		b, err := strconv.Atoi(tail)
		if err != nil {
			return 0, 0, 0, err
		}
		return a, b, 0, nil
	}
	n, err := strconv.Atoi(s)
	return n, n, 0, err
}

func collectVerses(book string, startCh, endCh, startVs, endVs, crossEndCh int) []Verse {
	chapters := index[book]
	if chapters == nil {
		return nil
	}
	chs := sortedChapters(chapters)

	var result []Verse

	// Cross-chapter range: startCh:startVs .. crossEndCh:endVs
	if crossEndCh > 0 {
		for _, c := range chs {
			if c < startCh {
				continue
			}
			if c > crossEndCh {
				break
			}
			for _, v := range sortedVerses(chapters[c]) {
				if c == startCh && v < startVs {
					continue
				}
				if c == crossEndCh && v > endVs {
					break
				}
				result = append(result, allVerses[chapters[c][v]])
			}
			if c == crossEndCh {
				break
			}
		}
		return result
	}

	// Chapter range (or single), apply verse range within each chapter
	for _, c := range chs {
		if c < startCh {
			continue
		}
		if c > endCh {
			break
		}
		for _, v := range sortedVerses(chapters[c]) {
			if v < startVs {
				continue
			}
			if v > endVs {
				break
			}
			result = append(result, allVerses[chapters[c][v]])
		}
	}
	return result
}

func sortedChapters(m map[int]map[int]int) []int {
	out := make([]int, 0, len(m))
	for c := range m {
		out = append(out, c)
	}
	sort.Ints(out)
	return out
}

func sortedVerses(m map[int]int) []int {
	out := make([]int, 0, len(m))
	for v := range m {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}

func respondWithVerses(w http.ResponseWriter, vs []Verse) {
	const maxVerses = 2000 // safety cap
	if len(vs) == 0 {
		http.Error(w, "no verses matched", http.StatusNotFound)
		return
	}
	if len(vs) > maxVerses {
		http.Error(w,
			fmt.Sprintf("range too large (%d verses); limit is %d", len(vs), maxVerses),
			http.StatusBadRequest)
		return
	}
	writeJSON(w, vs)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Printf("json encode error: %v", err)
	}
}
