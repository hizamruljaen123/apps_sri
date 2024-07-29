package main

import (
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

// Struktur untuk gejala dan probabilitas
type Symptom struct {
	Gejala       string  `json:"Gejala"`
	Probabilitas float64 `json:"Probabilitas"`
}

// Fungsi untuk mendapatkan massa dari dataframe berdasarkan gejala
func getMass(df [][]string, symptom string) map[string]float64 {
	mass := make(map[string]float64)
	totalMass := 0.0
	for _, row := range df {
		if row[1] == symptom {
			var weightValue, importanceValue float64
			if _, err := fmt.Sscanf(row[2], "%f", &weightValue); err != nil {
				log.Printf("Error parsing weight value: %v", err)
				continue
			}
			if _, err := fmt.Sscanf(row[3], "%f", &importanceValue); err != nil {
				log.Printf("Error parsing importance value: %v", err)
				continue
			}
			mass[row[0]] = weightValue * importanceValue
			totalMass += mass[row[0]]
		}
	}

	// Normalisasi
	for k := range mass {
		mass[k] /= totalMass
	}

	// Menambahkan nilai theta (frame of discernment)
	mass["θ"] = 1 - totalMass

	return mass
}

// Fungsi untuk menghitung fungsi massa
func calculateMassFunctions(symptoms []string, df [][]string) []map[string]float64 {
	var masses []map[string]float64
	for _, symptom := range symptoms {
		symptomMass := getMass(df, symptom)
		masses = append(masses, symptomMass)
	}
	return masses
}

// Fungsi untuk menggabungkan kepercayaan
func combineBeliefs(mass1, mass2 map[string]float64) map[string]float64 {
	combinedMass := make(map[string]float64)
	conflictMass := 0.0

	for s1, m1 := range mass1 {
		for s2, m2 := range mass2 {
			if s1 == "θ" && s2 == "θ" {
				combinedMass["θ"] += m1 * m2
			} else if s1 == "θ" {
				combinedMass[s2] += m1 * m2
			} else if s2 == "θ" {
				combinedMass[s1] += m1 * m2
			} else if s1 == s2 {
				combinedMass[s1] += m1 * m2
			} else {
				conflictMass += m1 * m2
			}
		}
	}

	// Penanganan konflik total
	if math.Abs(conflictMass-1) < 1e-10 {
		log.Println("Warning: Total conflict detected")
		return combinedMass // Mengembalikan massa tanpa normalisasi dalam kasus konflik total
	}

	// Normalisasi
	totalMass := 1 - conflictMass
	for k, v := range combinedMass {
		combinedMass[k] = v / totalMass
	}

	return combinedMass
}

func renderHTML(c *gin.Context, filename string) {
	tmpl, err := template.ParseFiles(filename)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error loading template")
		return
	}
	tmpl.Execute(c.Writer, nil)
}

func main() {
	r := gin.Default()

	// Serve static files from the "static" directory
	r.Static("/static", "./static")

	// Routing index untuk memastikan aplikasi berjalan
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Aplikasi sudah berjalan",
		})
	})

	// Routing untuk halaman dashboard
	r.GET("/dashboard", func(c *gin.Context) {
		renderHTML(c, "templates/dashboard.html")
	})

	// Routing untuk halaman tambah data
	r.GET("/tambah_data", func(c *gin.Context) {
		renderHTML(c, "templates/tambah_data.html")
	})

	// Routing untuk halaman data
	r.GET("/data", func(c *gin.Context) {
		renderHTML(c, "templates/data.html")
	})

	// Routing untuk halaman grafik
	r.GET("/grafik", func(c *gin.Context) {
		renderHTML(c, "templates/grafik.html")
	})

	// Routing untuk mendapatkan data dari Excel
	r.GET("/get_data", func(c *gin.Context) {
		// Membuka file Excel
		f, err := excelize.OpenFile("data.xlsx")
		if err != nil {
			log.Printf("Gagal membuka file Excel: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuka file Excel"})
			return
		}
		defer f.Close()

		// Membaca semua baris dari sheet pertama
		rows, err := f.GetRows("Sheet1")
		if err != nil {
			log.Printf("Gagal membaca baris dari sheet: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membaca sheet"})
			return
		}

		// Menyiapkan slice untuk menyimpan data
		var data []map[string]string
		for _, row := range rows[1:] { // Mulai dari index 1 untuk melewatkan header
			if len(row) < 4 {
				continue // Lewatkan baris yang tidak memiliki cukup kolom
			}
			entry := map[string]string{
				"nama_penyakit": row[0],
				"gejala":        row[1],
				"bobot_gejala":  row[2],
				"importance":    row[3],
			}
			data = append(data, entry)
		}

		// Mengembalikan data sebagai JSON
		c.JSON(http.StatusOK, data)
	})

	// Routing untuk memproses data menggunakan Dempster-Shafer
	r.POST("/process_data", func(c *gin.Context) {
		// Membuka file Excel
		f, err := excelize.OpenFile("data.xlsx")
		if err != nil {
			log.Printf("Gagal membuka file Excel: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuka file Excel"})
			return
		}
		defer f.Close()

		// Membaca semua baris dari sheet pertama
		rows, err := f.GetRows("Sheet1")
		if err != nil {
			log.Printf("Gagal membaca baris dari sheet: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membaca sheet"})
			return
		}

		// Membaca gejala dari body permintaan
		var selectedSymptoms struct {
			Symptoms []string `json:"symptoms"`
		}
		if err := c.ShouldBindJSON(&selectedSymptoms); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		// Validasi input
		if len(selectedSymptoms.Symptoms) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "At least one symptom is required"})
			return
		}

		// Validasi bahwa gejala yang dipilih ada dalam dataset
		validSymptoms := make(map[string]bool)
		for _, row := range rows[1:] { // Mulai dari index 1 untuk melewatkan header
			validSymptoms[row[1]] = true
		}
		for _, symptom := range selectedSymptoms.Symptoms {
			if !validSymptoms[symptom] {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid symptom: %s", symptom)})
				return
			}
		}

		// Menghitung fungsi massa
		masses := calculateMassFunctions(selectedSymptoms.Symptoms, rows)

		// Menggabungkan kepercayaan
		var combinedMass map[string]float64
		for i, mass := range masses {
			if i == 0 {
				combinedMass = mass
			} else {
				combinedMass = combineBeliefs(combinedMass, mass)
			}
		}

		// Menyiapkan hasil akhir dengan probabilitas tertinggi
		maxBelief := ""
		maxProb := 0.0
		for k, v := range combinedMass {
			if k != "θ" && v > maxProb {
				maxProb = v
				maxBelief = k
			}
		}

		// Mengembalikan hasil sebagai JSON
		c.JSON(http.StatusOK, gin.H{
			"Penyakit":     maxBelief,
			"Probabilitas": fmt.Sprintf("%.4f", maxProb*100),
			"DetailMassa":  combinedMass,
		})
	})

	// Menjalankan server pada port 8080
	r.Run(":8080")
}
