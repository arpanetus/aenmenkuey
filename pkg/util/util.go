package util

func AppendTrailingSlash(url string) string {
	rurl := []rune(url)
	if rurl[len(rurl)-1] != '/' {
		return string(append(rurl, '/'))
	}
	return url
}
