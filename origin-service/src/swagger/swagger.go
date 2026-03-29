package swagger

import (
	"embed"
	"net/http"
)

//go:embed *
var staticFS embed.FS

//go:embed swagger.json
var specYAML []byte

// RegisterHandlers регистрирует HTTP-обработчики для Swagger UI и самой спецификации.
// После вызова этой функции документация становится доступна по адресу /docs/.
//
// Пример использования в main.go:
//
//	mux := http.NewServeMux()
//	swagger.RegisterHandlers(mux)
//	log.Fatal(http.ListenAndServe(":8080", mux))
func RegisterHandlers(mux *http.ServeMux) {
	fs := http.FileServer(http.FS(staticFS))
	mux.Handle("/docs/", http.StripPrefix("/docs/", fs))

	mux.HandleFunc("/docs/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		_, _ = w.Write(specYAML)
	})

	// Редирект с /docs на /docs/
	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/", http.StatusMovedPermanently)
	})
}