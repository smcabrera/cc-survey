package views

import (
	"log"
	"net/http"
	"runtime/debug"
)

import (
    "github.com/julienschmidt/httprouter"
)

import (
	"github.com/timtadh/cc-survey/models"
)


type Context struct {
	views *Views
	s *models.Session
	u *models.User
	rw http.ResponseWriter
	r *http.Request
	p httprouter.Params
}

func (v *Views) Context(f View) httprouter.Handle {
	return func(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
		defer func() {
			if e := recover(); e != nil {
				log.Println(e)
				log.Println(string(debug.Stack()))
			}
			return
		}()
		c := &Context{
			views: v,
			rw: rw, r: r, p: p,
		}
		c.Session(v.Log(f))
	}
}

func (v *Views) LoggedOut(f View, to string) View {
	return func(c *Context) {
		if c.u != nil {
			err := c.s.Invalidate(v.sessions, c.rw)
			if err != nil {
				log.Println(err)
			}
			http.Redirect(c.rw, c.r, to, 302)
		} else {
			f(c)
		}
	}
}

func (v *Views) LoggedIn(f View) View {
	return func(c *Context) {
		if c.u == nil {
			http.Redirect(c.rw, c.r, "/login", 302)
		} else {
			f(c)
		}
	}
}

func (v *Views) LoggedInRedirect(f View, to string) View {
	return func(c *Context) {
		if c.u != nil {
			http.Redirect(c.rw, c.r, to, 302)
		} else {
			f(c)
		}
	}
}

func (c *Context) Session(f View) {
	doErr := func(c *Context, err error) {
		log.Println(err)
		c.rw.WriteHeader(500)
		c.rw.Write([]byte("error processing request"))
	}
	s, err := models.GetSession(c.views.sessions, c.rw, c.r)
	if err != nil {
		doErr(c, err)
	}
	c.s = s
	if s.User != "" {
		u, err := c.views.users.Get(s.User)
		if err != nil {
			doErr(c, err)
		}
		c.u = u
	}
	f(c)
}

func (c *Context) SetUser(u *models.User) error {
	c.u = u
	c.s.User = u.Email
	return c.views.sessions.Update(c.s)
}

