package export

type Option func(*options)

type options struct {
	cookies []map[string]interface{}
	url     string
	name    string
	screen  bool
	parse   bool
	column  int
}

func WithCookies(cookies []map[string]interface{}) Option {
	return func(o *options) {
		o.cookies = cookies
	}
}

func WithUrl(url string) Option {
	return func(o *options) {
		o.url = url
	}
}

func WithName(name string) Option {
	return func(o *options) {
		o.name = name
	}
}

func WithScreen(screen bool) Option {
	return func(o *options) {
		o.screen = screen
	}
}

func WithParse(parse bool) Option {
	return func(o *options) {
		o.parse = parse
	}
}

func WithColumn(column int) Option {
	return func(o *options) {
		o.column = column
	}
}
