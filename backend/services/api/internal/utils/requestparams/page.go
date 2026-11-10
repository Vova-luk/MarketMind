package requestparams

import (
	"math"
	"net/url"
	"strconv"
)

type Page struct {
	Number int
	Size   int
}

func ParsePage(rawQuery string) (Page, bool) {
	params, err := url.ParseQuery(rawQuery)
	if err != nil {
		return Page{}, false
	}

	page := Page{Number: 1, Size: 10}
	for key, values := range params {
		if len(values) != 1 {
			return Page{}, false
		}
		value, err := strconv.ParseInt(values[0], 10, 32)
		if err != nil || value < 1 {
			return Page{}, false
		}
		switch key {
		case "page":
			page.Number = int(value)
		case "page_size":
			if value > 100 {
				return Page{}, false
			}
			page.Size = int(value)
		default:
			return Page{}, false
		}
	}
	return page, true
}

func (p Page) Offset() int64 {
	return int64(p.Number-1) * int64(p.Size)
}

func (p Page) NextPage(fetched int) *int {
	if fetched <= p.Size || p.Number == math.MaxInt32 {
		return nil
	}
	next := p.Number + 1
	return &next
}
