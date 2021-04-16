package export

import (
	"encoding/csv"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/HarryBird/cdp"
	xcdp "github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

var (
	tdRegexp = regexp.MustCompile(`<td[^>]*>\s*(.*)\s*<\/td[^>]*>`)
	aRegexp  = regexp.MustCompile(`<[^>]+>`)
)

type Export struct {
	opts    options
	logger  *log.Logger
	records [][]string
}

func New(opts ...Option) *Export {
	options := options{}

	for _, o := range opts {
		o(&options)
	}

	return &Export{
		opts:    options,
		logger:  log.New(os.Stdout, "[EXPORTER] ", log.LstdFlags|log.Lshortfile|log.Lmsgprefix),
		records: [][]string{[]string{"交易金额", "说明", "账户余额", "交易时间"}},
	}
}

func (e *Export) Run() error {
	if err := e.handleCookies(); err != nil {
		return errors.WithMessage(err, "Run: handle cookies fail")
	}

	if e.opts.column == 4 {
		e.records = [][]string{[]string{"交易金额", "说明", "账户余额", "交易时间"}}
	} else if e.opts.column == 3 {
		e.records = [][]string{[]string{"交易金额", "说明", "交易时间"}}
	}

	var buf string
	var screen []byte
	page := 1
	size := 10

	for {
		url := e.opts.url + "page=" + strconv.Itoa(page) + "&size=" + strconv.Itoa(size)
		e.logger.Printf("%s %s %s", "[INFO] ", "render url -> ", url)

		if err := e.html(url, &buf); err != nil {
			return errors.WithMessagef(err, "Run: render html fail -> %s", url)
		}

		e.logger.Printf("%s %s", "[INFO] ", "parse html...")
		records, err := e.parse(&buf)

		if err != nil {
			return errors.WithMessage(err, "Run: parse html fail")
		}

		if len(records) == 0 {
			e.logger.Printf("%s %s", "[INFO] ", "no next page...")
			break
		}

		e.logger.Printf("%s %s", "[INFO] ", "get some records...")
		e.records = append(e.records, records...)

		if e.opts.screen {
			e.logger.Printf("%s %s", "[INFO] ", "screen...")
			if err := e.screen(url, &screen); err != nil {
				return errors.WithMessagef(err, "Run: get screen fail -> %s", url)
			}

			e.logger.Printf("%s %s %s", "[INFO] ", "store screen file ...", "page-"+strconv.Itoa(page))
			if err := e.store(&screen, page); err != nil {
				return errors.WithMessagef(err, "Run: store screen fail -> %s", url)
			}
		}

		/*
			if page%3 == 0 {
				break
			}
		*/

		time.Sleep(500 * time.Millisecond)
		page += 1
	}

	if e.opts.parse {
		if err := e.csv(); err != nil {
			return errors.WithMessagef(err, "Run: export to csv fail")
		}
	}

	// e.logger.Printf("%s %s %+v", "[DEBUG] ", "all records", e.records)
	e.logger.Printf("%s %s", "[INFO] ", "DONE~")

	return nil
}

func (e *Export) csv() error {
	dir := "./lantouzi/" + e.opts.name + "/"

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file := dir + "record.csv"

	f, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		return err
	}

	defer f.Close()

	f.WriteString("\xEF\xBB\xBF")

	w := csv.NewWriter(f)

	if err := w.WriteAll(e.records); err != nil {
		return err
	}

	w.Flush()

	return nil
}

func (e *Export) store(buf *[]byte, page int) error {
	dir := "./lantouzi/" + e.opts.name + "/"

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file := dir + "page-" + strconv.Itoa(page) + ".png"

	if err := ioutil.WriteFile(file, *buf, 0755); err != nil {
		return err
	}

	return nil
}

func (e *Export) screen(url string, buf *[]byte) error {
	helper := cdp.NewHelper(url).WithInfoLogger(log.Printf).WithErrorLogger(log.Printf)

	if err := helper.WithCookies(e.opts.cookies); err != nil {
		return err
	}

	if err := helper.FullScreen(100, buf); err != nil {
		return err
	}

	return nil

}

func (e *Export) html(url string, buf *string) error {
	helper := cdp.NewHelper(url).WithInfoLogger(log.Printf).WithErrorLogger(log.Printf)

	if err := helper.WithCookies(e.opts.cookies); err != nil {
		return err
	}

	if err := helper.InnerHTML(
		`document.querySelector("body > div.g-uc-page.clearfix > div.g-uc-main > div > div.bd > div:nth-child(2) > table")`,
		buf, chromedp.NodeVisible, chromedp.ByJSPath,
	); err != nil {
		return err
	}

	return nil
}

func (e *Export) parse(buf *string) ([][]string, error) {
	records := [][]string{}

	match := tdRegexp.FindAllStringSubmatch(*buf, -1)

	if len(match) > 0 {
		record := []string{}
		for i, v := range match {
			idx := i + 1
			val := aRegexp.ReplaceAllString(strings.TrimSpace(v[1]), "")
			//e.logger.Printf("%s %s %+v", "[DEBUG] ", "Trim After", val)

			record = append(record, val)

			if idx%e.opts.column == 0 {
				records = append(records, record)
				record = []string{}
			}
		}
	}

	//e.logger.Printf("match: %v", records)

	return records, nil
}

func (e *Export) handleCookies() error {
	if len(e.opts.cookies) == 0 {
		return nil
	}

	for i, _ := range e.opts.cookies {
		exp, ok := e.opts.cookies[i]["ValidTime"].(int)

		if !ok {
			return errors.New("handleCookie: cookie 's expires invalid")
		}
		e.opts.cookies[i]["Expires"] = xcdp.TimeSinceEpoch(time.Now().Add(time.Duration(exp) * time.Second))
	}

	return nil
}
