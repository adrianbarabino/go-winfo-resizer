package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"resizer/controllers" // Cambiar según tu módulo
)

func main() {
	fmt.Println("Iniciando el programa...") // Esto debería imprimirse

	// Crear el router
	r := mux.NewRouter()
	resizeController := &controllers.ResizeController{}

	// Definir la ruta /resize
	r.HandleFunc("/resize", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Endpoint /resize alcanzado") // Para debug
		params := r.URL.Query()

		// Obtener parámetros de la URL
		filename := params.Get("filename")
		width := params.Get("width")
		height := params.Get("height")
		quality := params.Get("quality")
		crop := params.Get("crop")

		// Llamar al método del controlador
		resizeController.ResizeImage(w, r, filename, quality, width, height, crop)
	}).Methods("GET")

	// Configuración del servidor
	port := ":8087"
	log.Printf("Servidor iniciado en http://localhost%s\n", port)
	if err := http.ListenAndServe(port, r); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}
