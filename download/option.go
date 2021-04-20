package download

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
