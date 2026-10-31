package drive

import (
	"encoding/base64"
	"strconv"
)

// A listing is returned in pages (app standard rows 15 and 25): limit 1..1000 (default 1000) and an opaque
// cursor holding the position in the sorted listing. A folder holds at most 10,000 entries (List).
const MaxPage = 1000

func pageEntries(all []Entry, cursor, limit string) ([]Entry, *string, error) {
	size := MaxPage
	if limit != "" {
		n, err := strconv.Atoi(limit)
		if err != nil || n < 1 || n > MaxPage {
			return nil, nil, problem(400, "INVALID_REQUEST", "Giới hạn trang không hợp lệ.")
		}
		size = n
	}
	start := 0
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		n, err2 := strconv.Atoi(string(raw))
		if err != nil || err2 != nil || n < 0 || n > len(all) {
			return nil, nil, problem(400, "INVALID_CURSOR", "Con trỏ trang không hợp lệ.")
		}
		start = n
	}
	end := min(start+size, len(all))
	if end == len(all) {
		return all[start:end], nil, nil
	}
	next := base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(end)))
	return all[start:end], &next, nil
}
