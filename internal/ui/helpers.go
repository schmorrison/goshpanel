package ui

import "net/url"

func urlQueryEscape(value string) string {
	return url.QueryEscape(value)
}
