package prospect

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type Resolver interface {
	Resolve(Data) string
	fmt.Stringer
}

type Pattern struct {
	Resolver
}

func (p *Pattern) Set(str string) error {
	r, err := ParseResolver(str)
	if err == nil {
		p.Resolver = r
	}
	return err
}

func ParseResolver(str string) (Resolver, error) {
	if str == "" {
		return empty{}, nil
	}
	var (
		rs    []Resolver
		parts = strings.Split(strings.Trim(str, "/"), "/")
	)
	for _, p := range parts {
		r, err := parse(p)
		if err != nil {
			return nil, err
		}
		rs = append(rs, r)
	}

	return path{rs: rs}, nil
}

const (
	lcurly = '{'
	rcurly = '}'
	colon  = ':'
)

const (
	levelLevel    = "level"
	levelSource   = "source"
	levelModel    = "model"
	levelMime     = "mime"
	levelFormat   = "format"
	levelType     = "type"
	levelRun      = "run"
	levelYear     = "year"
	levelDoy      = "doy"
	levelMonth    = "month"
	levelDay      = "day"
	levelHour     = "hour"
	levelMinLong  = "minute"
	levelMinShort = "min"
	levelSecLong  = "second"
	levelSecShort = "sec"
	levelStamp    = "timestamp"
)

func parse(str string) (Resolver, error) {
	var (
		offset int
		rs     []Resolver
	)
	for offset < len(str) {
		start := strings.IndexByte(str[offset:], lcurly)
		if start < 0 {
			break
		}
		end := strings.IndexByte(str[offset+start:], rcurly)
		if end < 0 {
			return nil, fmt.Errorf("missing closing brace")
		}
		if end-start == 1 {
			return nil, fmt.Errorf("empty placeholder")
		}

		if q := str[offset : offset+start]; len(q) > 0 {
			rs = append(rs, literal(q))
		}
		offset += start + 1
		r, err := parseResolver(str[offset : offset+end-1])
		if err != nil {
			return nil, err
		}
		rs = append(rs, r)

		offset += end
	}

	if len(str[offset:]) > 0 {
		rs = append(rs, literal(str[offset:]))
	}

	if len(rs) == 1 {
		return rs[0], nil
	}
	return compound{rs: rs}, nil
}

func parseResolver(str string) (Resolver, error) {
	var err error
	if !(isNumber(str[0]) || isSign(str[0])) {
		return fragment{name: str}, err
	}
	x := strings.IndexByte(str, colon)
	if x < 0 {
		var i index
		i.index, err = strconv.Atoi(str)
		return i, err
	}
	var i slice
	if i.begin, err = strconv.Atoi(str[:x]); err != nil {
		return nil, err
	}
	if i.end, err = strconv.Atoi(str[x+1:]); err != nil {
		return nil, err
	}
	return i, err
}

type empty struct{}

func (e empty) Resolve(d Data) string {
	return "" //filepath.Base(d.File)
}

func (e empty) String() string {
	return ""
}

type path struct {
	rs []Resolver
}

func (p path) Resolve(dat Data) string {
	str := make([]string, len(p.rs))
	for j := range p.rs {
		str[j] = p.rs[j].Resolve(dat)
	}
	return filepath.Join(str...)
}

func (p path) String() string {
	str := make([]string, len(p.rs))
	for j := range p.rs {
		str[j] = p.rs[j].String()
	}
	return fmt.Sprintf("path(%s)", filepath.Join(str...))
}

type literal string

func (i literal) Resolve(_ Data) string {
	return string(i)
}

func (i literal) String() string {
	return fmt.Sprintf("literal(%s)", string(i))
}

type index struct {
	index int
}

func (i index) Resolve(dat Data) string {
	var (
		dir = filepath.Dir(dat.File)
		xs  = strings.Split(strings.TrimPrefix(dir, "/"), "/")
		str string
	)
	if i.index < len(xs) {
		str = xs[i.index]
	}
	return str
}

func (i index) String() string {
	return fmt.Sprintf("index(%d)", i.index)
}

type slice struct {
	begin int
	end   int
}

func (i slice) Resolve(dat Data) string {
	var (
		dir   = filepath.Dir(dat.File)
		xs    = strings.Split(strings.TrimPrefix(dir, "/"), "/")
		begin = normalize(i.begin, len(xs))
		end   = normalize(i.end, len(xs))
		str   string
	)
	switch {
	case end == begin:
		str = xs[begin]
	case end > begin:
		str = filepath.Join(xs[begin:end]...)
	}
	return str
}

func (i slice) String() string {
	return fmt.Sprintf("range(%d:%d)", i.begin, i.end)
}

func normalize(index, size int) int {
	if index < 0 {
		index = size + index
	}
	if index < 0 {
		index = 0
	} else if index >= size {
		index = size
	}
	return index
}

func isNumber(char byte) bool {
	return (char >= '0' && char <= '9')
}

func isSign(char byte) bool {
	return char == '-'
}

type fragment struct {
	name string
}

func (f fragment) Resolve(dat Data) string {
	replace := func(str string) string {
		return strings.ReplaceAll(strings.Title(str), " ", "")
	}

	var str string
	switch strings.ToLower(f.name) {
	default:
		if x, err := strconv.Atoi(f.name); err == nil {
			var (
				dir = filepath.Dir(f.name)
				xs  = strings.Split(strings.TrimPrefix(dir, "/"), "/")
			)
			x--
			if x < len(xs) {
				str = xs[x]
			}
		}
	case levelRun:
		str = dat.Source
	case levelLevel:
		str = strconv.Itoa(dat.Level)
	case levelSource:
		str = replace(dat.Source)
	case levelModel:
		str = replace(dat.Model)
	case levelMime, levelFormat:
		str = replace(splitMime(dat.Mime))
	case levelType:
		str = replace(dat.Type)
	case levelYear:
		str = strconv.Itoa(dat.AcqTime.Year())
	case levelDoy:
		str = fmt.Sprintf("%03d", dat.AcqTime.YearDay())
	case levelMonth:
		str = fmt.Sprintf("%02d", dat.AcqTime.Month())
	case levelDay:
		str = fmt.Sprintf("%02d", dat.AcqTime.Day())
	case levelHour:
		str = fmt.Sprintf("%02d", dat.AcqTime.Hour())
	case levelMinShort, levelMinLong:
		str = fmt.Sprintf("%02d", dat.AcqTime.Minute())
	case levelSecShort, levelSecLong:
		str = fmt.Sprintf("%02d", dat.AcqTime.Second())
	case levelStamp:
		str = strconv.Itoa(int(dat.AcqTime.Unix()))
	}
	return str
}

func (f fragment) String() string {
	return fmt.Sprintf("fragment(%s)", f.name)
}

type compound struct {
	rs []Resolver
}

func (c compound) Resolve(dat Data) string {
	var buf strings.Builder
	for _, r := range c.rs {
		buf.WriteString(r.Resolve(dat))
	}
	return buf.String()
}

func (c compound) String() string {
	var buf strings.Builder
	for _, r := range c.rs {
		buf.WriteString(r.String())
	}
	return fmt.Sprintf("compound(%s)", buf.String())
}

func splitMime(mime string) string {
	if ix := strings.Index(mime, "/"); ix >= 0 && ix+1 < len(mime) {
		mime = mime[ix+1:]
	}
	if ix := strings.Index(mime, ";"); ix >= 0 {
		mime = mime[:ix]
	}
	return mime
}
