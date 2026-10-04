package template

import "strings"

const Default = "【pushotp】验证码：{code}，{ttl} 内有效。"

type Data struct {
	Code     string
	TTL      string
	Scene    string
	Receiver string
	Extra    map[string]string
}

func Render(tmpl string, d Data) string {
	if tmpl == "" {
		tmpl = Default
	}
	pairs := []string{
		"{code}", d.Code,
		"{ttl}", d.TTL,
		"{scene}", d.Scene,
		"{receiver}", d.Receiver,
	}
	for k, v := range d.Extra {
		pairs = append(pairs, "{"+k+"}", v)
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}
