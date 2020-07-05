package main

import (
	"main/components/materialize/components"

	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
)

type Homepage struct {
	WebPage
}

var Home = &Homepage{
	WebPage: WebPage{
		Name:  "Home - Gosh Panel \"Oh My\" ",
		Items: nil,
	},
}

func (h *Homepage) Render() vecty.ComponentOrHTML {
	c := components.Card{
		Content: nil,
		Class:   []string{"super-class"},
		Title:   "CARD TITLE",
	}
	return elem.Section(
		c.Render(),
	)
}
