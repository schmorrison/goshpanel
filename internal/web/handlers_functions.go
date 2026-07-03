package web

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/schmorrison/goshpanel/internal/fn"
	"github.com/schmorrison/goshpanel/internal/store"
)

type functionsListData struct {
	Functions []store.MicroFunction
	BaseURL   string
}

type functionEditData struct {
	Function store.MicroFunction
	InvokeURL string
	Result   *fn.InvokeResult
}

func (s *Server) handleFunctionsPage(w http.ResponseWriter, r *http.Request) {
	if s.fns == nil {
		redirectError(w, r, "/", moduleDisabled("functions"))
		return
	}
	list, err := s.store.MicroFunctions()
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	s.render(w, r, "functions.html", "Micro Functions", "functions", functionsListData{
		Functions: list,
		BaseURL:   requestBaseURL(r),
	})
}

func (s *Server) handleFunctionCreate(w http.ResponseWriter, r *http.Request) {
	if s.fns == nil {
		redirectError(w, r, "/functions", moduleDisabled("functions"))
		return
	}
	timeout, _ := strconv.Atoi(r.FormValue("timeout_sec"))
	f, err := s.fns.Create(r.FormValue("name"), r.FormValue("description"), r.FormValue("script"), timeout)
	if err != nil {
		redirectError(w, r, "/functions", err)
		return
	}
	s.audit(r, "functions.create", f.Name)
	redirectFlash(w, r, "/functions", "Function created: "+f.Name)
}

func (s *Server) handleFunctionEditPage(w http.ResponseWriter, r *http.Request) {
	if s.fns == nil {
		redirectError(w, r, "/functions", moduleDisabled("functions"))
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		redirectError(w, r, "/functions", err)
		return
	}
	f, err := s.store.MicroFunctionByID(id)
	if err != nil {
		redirectError(w, r, "/functions", err)
		return
	}
	s.render(w, r, "function_edit.html", "Edit "+f.Name, "functions", functionEditData{
		Function:  f,
		InvokeURL: fn.InvokeURL(requestBaseURL(r), f),
	})
}

func (s *Server) handleFunctionSave(w http.ResponseWriter, r *http.Request) {
	if s.fns == nil {
		redirectError(w, r, "/functions", moduleDisabled("functions"))
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		redirectError(w, r, "/functions", err)
		return
	}
	timeout, _ := strconv.Atoi(r.FormValue("timeout_sec"))
	enabled := r.FormValue("enabled") == "on"
	if err := s.fns.Update(id, r.FormValue("description"), r.FormValue("script"), enabled, timeout); err != nil {
		redirectError(w, r, "/functions/"+r.PathValue("id")+"/edit", err)
		return
	}
	s.audit(r, "functions.save", r.PathValue("id"))
	redirectFlash(w, r, "/functions/"+r.PathValue("id")+"/edit", "Function saved")
}

func (s *Server) handleFunctionDelete(w http.ResponseWriter, r *http.Request) {
	if s.fns == nil {
		redirectError(w, r, "/functions", moduleDisabled("functions"))
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		redirectError(w, r, "/functions", err)
		return
	}
	f, _ := s.store.MicroFunctionByID(id)
	if err := s.fns.Delete(id); err != nil {
		redirectError(w, r, "/functions", err)
		return
	}
	s.audit(r, "functions.delete", f.Name)
	redirectFlash(w, r, "/functions", "Function deleted")
}

func (s *Server) handleFunctionInvokePanel(w http.ResponseWriter, r *http.Request) {
	if s.fns == nil {
		redirectError(w, r, "/functions", moduleDisabled("functions"))
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		redirectError(w, r, "/functions", err)
		return
	}
	f, err := s.store.MicroFunctionByID(id)
	if err != nil {
		redirectError(w, r, "/functions", err)
		return
	}
	res, err := s.fns.Invoke(r.Context(), f.Name, f.Token)
	data := functionEditData{Function: f, InvokeURL: fn.InvokeURL(requestBaseURL(r), f)}
	if err != nil {
		data.Result = &fn.InvokeResult{Name: f.Name, Output: err.Error(), ExitCode: -1}
	} else {
		data.Result = &res
	}
	s.audit(r, "functions.invoke", f.Name)
	s.render(w, r, "function_edit.html", "Edit "+f.Name, "functions", data)
}

func (s *Server) handleFunctionInvokePublic(w http.ResponseWriter, r *http.Request) {
	if s.fns == nil {
		http.Error(w, "functions disabled", http.StatusServiceUnavailable)
		return
	}
	name := r.PathValue("name")
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("X-Function-Token")
	}
	res, err := s.fns.Invoke(r.Context(), name, token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Function-Exit-Code", fmt.Sprintf("%d", res.ExitCode))
	w.Header().Set("X-Function-Elapsed", res.Elapsed.String())
	if res.TimedOut {
		w.WriteHeader(http.StatusGatewayTimeout)
	}
	w.Write([]byte(res.Output))
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	return scheme + "://" + r.Host
}
