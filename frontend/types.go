package main

import (
	"net/url"

	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
)

// WebPage is a single 'webpage' which can be routed to in a browser.
type WebPage struct {
	vecty.Core

	// Name is the title of the page
	Name string
	// Link is URL for accessing the page
	Link *url.URL
	// Items is a list of UI/data which are viewable on the WebPage they are in
	Items []*WebItem
}

// WebItem is a single piece of UI/data which can be displayed on a WebPage
type WebItemI interface {
	// Render will output the vecty element that will be used on the site
	Render() vecty.ComponentOrHTML

	// Listen will receive a channel which the WebItem listens to for data in order to render
	Listen(ch <-chan interface{})
}

type WebItem struct {
	Name string
	C    <-chan int64

	Data int64
}

func (m WebItem) Render() vecty.ComponentOrHTML {
	return elem.Section(
		elem.Div(
			vecty.Markup(vecty.Class("components", "blue-grey", "lighten-3")),
			elem.Div(
				vecty.Markup(vecty.Class("components-content")),
				elem.Paragraph(vecty.Text("This WebItem doesn't supply a Render function!")),
			),
		),
	)
}

func (m WebItem) Listen(ch <-chan int64) {
	m.C = ch
	m.Data = <-m.C
}

type ModuleCard struct {
	WebItem
}

func (m ModuleCard) Render() vecty.ComponentOrHTML {
	panic("implement me")
}

func (m ModuleCard) Listen(ch <-chan interface{}) {
	panic("implement me")
}
