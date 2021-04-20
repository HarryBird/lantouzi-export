package download

import (
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/HarryBird/cdp"
	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

type downItem map[string][]string
type downList map[string]downItem

type Download struct {
	opts   options
	logger *log.Logger
}

func New(opts ...Option) *Download {
	options := options{}

	for _, o := range opts {
		o(&options)
	}

	return &Download{
		opts:   options,
		logger: log.New(os.Stdout, "[DOWNLOAD] ", log.LstdFlags|log.Lshortfile|log.Lmsgprefix),
	}
}

func (self *Download) Run() error {
	downs := downList{}

	servs, err := self.getServices()

	if err != nil {
		return err
	}

	/*
		servs := map[string]string{
			"智选服务6月期D10482": "https://lantouzi.com/user/smartbid/order/detail?id=ltz5baf814f22b75181&smb_type=1",
		}
	*/

	for name, url := range servs {
		name, item, err := self.handleServ(name, url)
		if err != nil {
			return err
		}
		downs[name] = item
		// break
	}

	// self.logger.Printf("%s %s %s %+v", "[DEBUG]", "[Run]", "download map", downs)

	return self.store(downs)
}

func (self *Download) download(url, dir string, idx int) error {

	fileRegexp := regexp.MustCompile(`filename="([^"]+)"`)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return err
	}

	for _, c := range self.opts.cookies {
		var ck http.Cookie
		if err := mapstructure.Decode(c, &ck); err != nil {
			return errors.WithMessagef(err, "%s %s %v", "[Download]", "build cookie fail", c)
		}

		req.AddCookie(&ck)
	}

	r, err := client.Do(req)

	if err != nil {
		return err
	}

	defer r.Body.Close()

	cLen := r.ContentLength
	cPos := r.Header.Get("Content-Disposition")
	file := ""

	if cLen == 0 {
		self.logger.Printf("%s %s %s %s -> %s", "[WARN]", "[Download]", "invalid file, ignore...", dir, url)
		return nil
	}

	matches := fileRegexp.FindStringSubmatch(cPos)
	if len(matches) > 0 {
		file = strings.TrimSpace(matches[1])
	}

	if file == "" {
		return errors.Errorf("%s %s -> %s", "download file fail, filename empty", dir, url)
	}

	filename := dir + strconv.Itoa(idx) + "_" + file

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0755)

	if err != nil {
		return errors.WithMessagef(err, "%s %s -> %s", "[Download]", "create file fail", filename)
	}

	defer f.Close()

	_, err = io.Copy(f, r.Body)

	if err != nil {
		return errors.WithMessagef(err, "%s %s -> %s", "[Download]", "store file fail", filename)
	}

	self.logger.Printf("%s %s %s %s", "[INFO] ", "[Download]", "download file -> ", filename)

	return nil

}

func (self *Download) store(m downList) error {
	for folder, items := range m {
		dir := "./lantouzi/合同/" + folder
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.WithMessagef(err, "%s %s -> %s", "[Store]", "create dir fail", dir)
		}

		for name, urls := range items {
			idx := 0
			dir := dir + "/" + name + "/"

			if err := os.MkdirAll(dir, 0755); err != nil {
				return errors.WithMessagef(err, "%s %s -> %s", "[Store]", "create dir fail", dir)
			}

			self.logger.Printf("%s %s %s %s", "[INFO] ", "[Store]", "create dir -> ", dir)

			for _, url := range urls {
				idx += 1
				if err := self.download(url, dir, idx); err != nil {
					return err
				}
				time.Sleep(1 * time.Second)
			}
		}
	}

	return nil
}

func (self *Download) handleServ(name, url string) (string, downItem, error) {
	var buf string

	item := map[string][]string{}
	self.logger.Printf("%s %s %s", "[INFO] ", "[Prepare]", url)

	helper := cdp.NewHelper(url).WithInfoLogger(log.Printf).WithErrorLogger(log.Printf)

	if err := helper.WithCookies(self.opts.cookies); err != nil {
		return name, item, err
	}

	err := helper.
		Init().
		//WithAction(chromedp.WaitVisible(`document.querySelector("#buy_prj_relation_pager > div")`, chromedp.NodeVisible, chromedp.ByJSPath)).
		WithAction(chromedp.WaitVisible(`document.querySelector("#buy_prj_relation_list > tr:nth-child(1)")`, chromedp.NodeVisible, chromedp.ByJSPath)).
		WithAction(chromedp.InnerHTML(`document.querySelector("body > div.g-uc-page.clearfix.no-side > div > div.uc-order-detail")`, &buf, chromedp.NodeVisible, chromedp.ByJSPath)).
		Run()

	if err != nil {
		return name, item, errors.WithMessagef(err, "%s %s -> %s", "[Prepare]", "get service detail html", url)
	}

	// self.logger.Printf("%s %s %+v", "[DEBUG] ", "service html", buf)

	dom, err := goquery.NewDocumentFromReader(strings.NewReader(buf))

	if err != nil {
		return name, item, errors.WithMessagef(err, "%s %s -> %s", "[Prepare]", "load html to dom fail", url)
	}

	title := ""
	dom.Find("a[class=a-title]").Each(func(i int, titleNode *goquery.Selection) {
		title = strings.TrimSpace(titleNode.Text())
	})

	if title == "" {
		return name, item, errors.Errorf("%s %s -> %s", "[Prepare]", "get title fail", url)
	}

	if title == "智选服务" {
		self.logger.Printf("%s %s %s %v %v", "[WARN] ", "[Prepare]", "may be invalid service", name, url)
		return name + "[死链]", item, nil
	}

	dom.Find("div[class=clearfix] a").Each(func(i int, nameNode *goquery.Selection) {
		name := strings.TrimSpace(nameNode.Text())
		if name == "服务协议" {
			if href, exist := nameNode.Attr("href"); exist {
				item[name] = []string{"https://lantouzi.com" + href}
			}
		}
	})

	//self.logger.Printf("%s %s %s %d", "[DEBUG] ", "[Prepare]", "tr nodes num", dom.Find("#buy_prj_relation_list").Find("tr").Length())

	dom.Find("#buy_prj_relation_list>tr").Each(func(i int, trNode *goquery.Selection) {
		var name string
		trNode.Find("td").Each(func(i int, tdNode *goquery.Selection) {
			if i == 1 {
				if name = strings.TrimSpace(tdNode.Text()); name != "" {
					item[name] = []string{}
				}

				// self.logger.Printf("%s %s %s %v %v", "[DEBUG] ", "[Prepare]", "name", name, i)
			}
		})

		trNode.Find("div[class=details-panel] td a").Each(func(i int, linkNode *goquery.Selection) {
			if _, ok := item[name]; ok {
				if href, exist := linkNode.Attr("href"); exist {
					item[name] = append(item[name], "https://lantouzi.com"+strings.TrimSpace(href))
					// self.logger.Printf("%s %s %s %v", "[DEBUG] ", "[Prepare]", "link", href)
				}
			}
			// html, _ := linkNode.Html()
			// self.logger.Printf("%s %s %s %v %v", "[DEBUG] ", "[Prepare]", "link html", name, html)
		})

		// html, err := trNode.Html()
		// self.logger.Printf("%s %s %s %v %v", "[DEBUG] ", "[Prepare]", "tr html", html, err)
	})

	// self.logger.Printf("%s %s %s %v", "[DEBUG] ", "[Prepare]", "item", item)
	return name, item, nil
}

func (self *Download) getServices() (map[string]string, error) {
	var buf string

	page := 0
	serv := map[string]string{}
	target := "https://lantouzi.com/user/smartbid/order/datalist?status=3&"

	for {
		page += 1
		url := target + "page=" + strconv.Itoa(page)

		self.logger.Printf("%s %s %s %s", "[INFO] ", "[Get Service]", "render html -> ", url)

		helper := cdp.NewHelper(url).WithInfoLogger(log.Printf).WithErrorLogger(log.Printf)

		if err := helper.WithCookies(self.opts.cookies); err != nil {
			return serv, err
		}

		if err := helper.InnerHTML(
			`document.querySelector("body > div.g-uc-page.clearfix > div.g-uc-main > div")`,
			&buf, chromedp.NodeVisible, chromedp.ByJSPath,
		); err != nil {
			return serv, errors.WithMessagef(err, "%s %s -> %s", "[Get Service]", "get service html fail", url)
		}

		// self.logger.Printf("%s %s %+v", "[DEBUG] ", "service html", buf)

		dom, err := goquery.NewDocumentFromReader(strings.NewReader(buf))

		if err != nil {
			return serv, errors.WithMessagef(err, "%s %s -> %s", "[Get Service]", "load html to dom fail", url)
		}

		liNodes := dom.Find("li")

		if liNodes.Length() == 0 {
			break
		}

		liNodes.Each(func(i int, li *goquery.Selection) {
			name := ""
			url := ""

			li.Find("div[class=name]:first-child").Each(func(ii int, nameNode *goquery.Selection) {
				name = strings.TrimSpace(nameNode.Text())
			})

			li.Find("a[class~=actionBtn]").Each(func(ii int, urlNode *goquery.Selection) {
				v, exists := urlNode.Attr("href")

				if exists {
					url = strings.TrimSpace(v)
				}
			})

			if name != "" && url != "" {
				serv[name] = url
			}
		})
	}

	// self.logger.Printf("%s %s %v", "[DEBUG] ", "[Get Service]", "all service", serv)
	return serv, nil
}

/*
func (e *Export) Run() error {

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

*/
