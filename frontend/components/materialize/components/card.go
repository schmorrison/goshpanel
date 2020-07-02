package components

import (
	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
	"golang.org/x/net/html/atom"
)

type Card struct {
	Content vecty.ComponentOrHTML
	Class   []string
	Title   string
	Image   string
	Actions []CardAction
	Fab     CardFab
}

type CardAction struct {
	Title  string
	Link   string
	Events []*vecty.EventListener
}

type CardFab struct {
	Icon   string
	Colour string
	Link   string
	Events []*vecty.EventListener
}

func (c Card) Render() vecty.ComponentOrHTML {
	c.Class = append([]string{"components"}, c.Class...)
	return elem.Div(
		vecty.Markup(vecty.Class(c.Class...)),
		c.image(),
		elem.Div(
			vecty.Markup(vecty.Class("card-content")),
			vecty.If(c.Image != "", c.title()),
			c.Content,
			c.fab(),
		),
		c.actions(),
	)
}

func (c Card) image() vecty.ComponentOrHTML {
	if c.Image != "" {
		return elem.Div(
			vecty.Markup(vecty.Class("card-image")),
			elem.Image(
				vecty.Markup(
					vecty.Property(atom.Src.String(), c.Image),
				),
			),
			c.title(),
		)
	}
	return nil
}

func (c Card) title() vecty.ComponentOrHTML {
	if c.Title != "" {
		return elem.Span(
			vecty.Markup(vecty.Class("card-title")),
			vecty.Text(c.Title),
		)
	}
	return nil
}

func (c Card) actions() vecty.ComponentOrHTML {
	if len(c.Actions) > 0 {
		var anchors []vecty.ComponentOrHTML
		for _, action := range c.Actions {
			// Create an anchor tag for the action
			anchor := elem.Anchor(
				vecty.Markup(
					vecty.Property("href", action.Link),
				),
				vecty.Text(action.Title),
			)

			// Apply the events to the anchor tag
			if len(action.Events) > 0 {
				for _, event := range action.Events {
					event.Apply(anchor)
				}
			}

			anchors = append(anchors, anchor)
		}

		return elem.Div(
			vecty.Markup(vecty.Class("card-action")),
			vecty.If(true, anchors...),
		)
	}
	return nil
}

func (c Card) fab() vecty.ComponentOrHTML {
	if c.Fab.Icon != "" {
		// Create an anchor tag for the action
		anchor := elem.Anchor(
			vecty.Markup(vecty.Class("btn-floating", "halfway-fab", "waves-effect", "waves-light", c.Fab.Colour)),
			vecty.Markup(vecty.Property("href", c.Fab.Link)),
			elem.Italic(vecty.Text(c.Fab.Icon)),
		)

		// Apply the events to the anchor tag
		if len(c.Fab.Events) > 0 {
			for _, event := range c.Fab.Events {
				event.Apply(anchor)
			}
		}

		return elem.Div(
			vecty.Markup(vecty.Class("card-action")),
			anchor,
		)
	}
	return nil
}
