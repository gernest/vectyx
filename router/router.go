// +build js

// Package router implements a client-side router for the rendering of
// components with the Vecty GopherJS framework.
//
// The router supports nested routes, named parameters (`/users/:userID`) and
// wildcards (`/users/*`).
//
// Currently only hash routing is implemented, HTML 5 history (aka pushState)
// support may follow in a future update.
package router

import (
	"net/url"
	"strings"

	"github.com/gopherjs/gopherjs/js"
	"github.com/gopherjs/vecty"
)

// DefaultConfig for convenience.
var DefaultConfig = &Config{}

// HandlerFunc allows the use of in-line functions to produce content for routes.
type HandlerFunc func(Context) vecty.ComponentOrHTML

// Handler is a vecty.Component that implements the OnRoute event receiver.
type Handler interface {
	vecty.Component
	// OnRoute is called when route changes require updating the component.
	OnRoute(Context)
}

// Router is a client-side router that can handle the rendering of nested Vecty
// components.
type Router struct {
	config  *Config
	root    *route
	parent  *Router
	pattern string
	routes  []*route
	routers []*Router
}

// Handle registers the handler for a given pattern.
func (r *Router) Handle(pattern string, h Handler) {
	r.routes = append(r.routes, newRoute(r.canonical(pattern), h))
}

// HandleFunc registers the handler function for a given pattern.
func (r *Router) HandleFunc(pattern string, f HandlerFunc) {
	r.routes = append(r.routes, newRoute(r.canonical(pattern), f))
}

// Group registers the root handler for pattern and a set of nested routes.
func (r *Router) Group(pattern string, h Handler, group func(r *Router)) {
	router := New(r.config)
	router.parent = r
	router.pattern = pattern
	router.root = newRoute(r.canonical(pattern), h)
	r.routers = append(r.routers, router)
	group(router)
}

// GroupFunc registers the root handler function for pattern and a set of nested
// routes.
func (r *Router) GroupFunc(pattern string, f HandlerFunc, group func(r *Router)) {
	router := New(r.config)
	router.parent = r
	router.pattern = pattern
	router.root = newRoute(r.canonical(pattern), f)
	r.routers = append(r.routers, router)
	group(router)
}

// Body returns the router result wrapped in a body tag, to be passed to
// vecty.RenderBody()
func (r *Router) Body() vecty.Component {
	r.root = newRoute(r.canonical(r.pattern), &body{})
	return r.start().(vecty.Component)
}

// Render returns the router result
func (r *Router) Render() vecty.ComponentOrHTML {
	return r.start()
}

// canonnical walks up the chain of nested routers to produce the canonical
// pattern for the current level
func (r *Router) canonical(pattern string) string {
	return r.walkPath() + pattern
}

// walkPath recursively builds the pattern
func (r *Router) walkPath() string {
	var path string
	if r.parent != nil {
		path = r.parent.walkPath()
	}
	path += r.pattern
	if path == `/` {
		return ``
	}

	return path
}

// hash obtains the current browser location hash component
func (r *Router) hash() string {
	return js.Global.Get(`location`).Get(`hash`).String()
}

// currentPath extracts the path component from the current browser location hash
func (r *Router) currentPath() string {
	return strings.SplitN(r.hash(), `#`, 2)[1]
}

// match walks the router returns the matching components for the current path
func (r *Router) match(path string) vecty.ComponentOrHTML {
	var children vecty.ComponentOrHTML
	var result vecty.ComponentOrHTML

	// Recursively render child routes
	for _, router := range r.routers {
		if children = router.match(path); children != nil {
			break
		}
	}

	// Render local routes
	var (
		max, score int
		winner     *route
		context, c *Context
	)
	for _, route := range r.routes {
		score, c = route.match(path)
		if score > max {
			max = score
			winner = route
			context = c
		}
	}
	if winner != nil {
		context.Children = children
		result = winner.render(context)
	}
	if result == nil && children != nil {
		result = children
	}

	if r.root != nil {
		if score, context = r.root.match(path); score > 0 {
			context.Children = result
			return r.root.render(context)
		}
	}

	return result
}

// update triggers a run of the router
func (r *Router) update() vecty.ComponentOrHTML {
	return r.match(r.currentPath())
}

// start initializes the router, ensures we have a hash component and attaches
// the event listener to trigger updates on hash change
func (r *Router) start() vecty.ComponentOrHTML {
	// Redirect to hash route if missing
	if r.hash() == `` {
		Go(`/`, nil)
	}
	js.Global.Call(`addEventListener`, `hashchange`, r.update, true)
	return r.update()
}

// Go transitions the browser to a new location
func Go(path string, params url.Values) {
	u, err := url.Parse(path)
	if err != nil {
		panic(err)
	}
	u.RawQuery = params.Encode()
	js.Global.Get(`location`).Set(`hash`, u.RequestURI())
}

// Link wraps the provided content in an anchor tag that transitions to a new
// location on click
func Link(path string, params url.Values, content vecty.ComponentOrHTML) vecty.ComponentOrHTML {
	return &anchor{path: path, params: params, children: content}
}

// New instantiates a new router.  If the config argument is nil, DefaultConfig
// will be used.
func New(config *Config) *Router {
	if config == nil {
		config = DefaultConfig
	}
	r := &Router{
		config:  config,
		routes:  make([]*route, 0),
		routers: make([]*Router, 0),
	}
	return r
}
