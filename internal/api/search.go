package api

import (
	"net/http"
	"net/url"
	"strconv"

	"gitea.mixdep.ru/mix/wynd/internal/search"
)

func (s *Server) handleCircleSearchAuthors(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	names, err := s.Search.SearchCircleAuthors(r.Context(), sess.AccountID, circleID, q.Get("q"), parseSearchFilters(q))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authors": names})
}

func (s *Server) handleCircleSearch(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	hits, err := s.Search.SearchCircle(r.Context(), sess.AccountID, circleID, q.Get("q"), limit, parseSearchFilters(q))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"hits": hits})
}

func (s *Server) handleGlobalSearch(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	filters := parseSearchFilters(q)
	// Автор — это лицо в круге, а не учётка: одно и то же имя в двух
	// кругах — разные люди. В общем поиске фильтр по автору не имеет смысла и
	// спекой не обещан («no author») — обнуляем его здесь.
	filters.Author = ""
	hits, err := s.Search.SearchAll(r.Context(), sess.AccountID, q.Get("q"), limit, filters)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"hits": hits})
}

func parseSearchFilters(q url.Values) search.Filters {
	return search.Filters{
		From:        q.Get("from"),
		To:          q.Get("to"),
		HasPhoto:    q.Get("has_photo") == "1",
		HasLocation: q.Get("has_location") == "1",
		Author:      q.Get("author"),
	}
}
