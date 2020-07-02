package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/gopherjs/vecty"
)

func main() {
	attachLocalStorage()

	vecty.SetTitle("Gosh Panel - Oh My")
	vecty.AddStylesheet("https://cdnjs.cloudflare.com/ajax/libs/xterm/3.14.5/xterm.min.css")
	vecty.AddStylesheet("https://fonts.googleapis.com/icon?family=Material+Icons")
	vecty.AddStylesheet("https://cdnjs.cloudflare.com/ajax/libs/materialize/1.0.0/css/materialize.min.css")

	p := &components.PageView{}
	store.Listeners.Add(p, func() {
		p.Items = store.Items
		vecty.Rerender(p)
	})
	vecty.RenderBody(p)
}

func attachLocalStorage() {
	store.Listeners.Add(nil, func() {
		data, err := json.Marshal(store.Items)
		if err != nil {
			println("failed to store items: " + err.Error())
		}
		js.Global().Get("localStorage").Set("items", string(data))
	})

	if data := js.Global().Get("localStorage").Get("items"); !data.IsUndefined() {
		var items []*model.Item
		if err := json.Unmarshal([]byte(data.String()), &items); err != nil {
			println("failed to load items: " + err.Error())
		}
		dispatcher.Dispatch(&actions.ReplaceItems{
			Items: items,
		})
	}
}
