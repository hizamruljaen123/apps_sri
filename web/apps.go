package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

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
	for _, row := range df {
		if row[1] == symptom {
			weight := row[2]
			var weightValue float64
			fmt.Sscanf(weight, "%f", &weightValue)
			mass[row[0]] = weightValue // Menggunakan penyakit sebagai kunci
		}
	}
	return mass
}

// Fungsi untuk menghitung fungsi massa
func calculateMassFunctions(dummySymptoms []Symptom, df [][]string) []map[string]float64 {
	var masses []map[string]float64
	for _, symptom := range dummySymptoms {
		gejala := symptom.Gejala
		probability := symptom.Probabilitas
		symptomMass := getMass(df, gejala)
		for k, v := range symptomMass {
			symptomMass[k] = v * probability
		}
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
			if s1 == s2 {
				combinedMass[s1] = combinedMass[s1] + m1*m2
			} else {
				combinedMass[s1+"|"+s2] = combinedMass[s1+"|"+s2] + m1*m2
			}
		}
	}
	// Normalisasi
	totalMass := 1 - conflictMass
	normalizedMass := make(map[string]float64)
	for k, v := range combinedMass {
		normalizedMass[k] = v / totalMass
	}
	return normalizedMass
}

func main() {
	r := gin.Default()

	// Routing index untuk memastikan aplikasi berjalan
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Aplikasi sudah berjalan",
		})
	})

	// Routing untuk mendapatkan data dari Excel
	r.GET("/get_data", func(c *gin.Context) {
		// Membuka file Excel
		f, err := excelize.OpenFile("data.xlsx")
		if err != nil {
			log.Fatalf("Gagal membuka file Excel: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuka file Excel"})
			return
		}
		defer f.Close()

		// Membaca semua baris dari sheet pertama
		rows, err := f.GetRows("Sheet1")
		if err != nil {
			log.Fatalf("Gagal membaca baris dari sheet: %v", err)
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
			log.Fatalf("Gagal membuka file Excel: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuka file Excel"})
			return
		}
		defer f.Close()

		// Membaca semua baris dari sheet pertama
		rows, err := f.GetRows("Sheet1")
		if err != nil {
			log.Fatalf("Gagal membaca baris dari sheet: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membaca sheet"})
			return
		}

		// Membaca gejala dari body permintaan
		var symptomGroups [][]Symptom
		if err := c.ShouldBindJSON(&symptomGroups); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		// Menyimpan hasil akhir untuk setiap grup gejala
		var results []map[string]float64
		for _, symptoms := range symptomGroups {
			// Menghitung fungsi massa
			masses := calculateMassFunctions(symptoms, rows)
			combinedMass := make(map[string]float64)
			for j, mass := range masses {
				if j == 0 {
					combinedMass = mass
				} else {
					combinedMass = combineBeliefs(combinedMass, mass)
				}
			}
			results = append(results, combinedMass)
		}

		// Menyiapkan hasil akhir dengan probabilitas tertinggi untuk setiap sampel
		var finalResults []map[string]interface{}
		for i, combinedMass := range results {
			if len(combinedMass) > 0 {
				maxBelief := ""
				maxProb := 0.0
				for k, v := range combinedMass {
					if v > maxProb {
						maxProb = v
						maxBelief = k
					}
				}
				// Pisahkan penyakit berdasarkan probabilitas tertinggi saja
				if strings.Contains(maxBelief, "|") {
					maxBelief = strings.Split(maxBelief, "|")[0]
				}
				finalResults = append(finalResults, map[string]interface{}{
					"Sample":       fmt.Sprintf("Sample %d", i+1),
					"Penyakit":     maxBelief,
					"Probabilitas": fmt.Sprintf("%.4f", maxProb*100),
				})

			} else {
				finalResults = append(finalResults, map[string]interface{}{
					"Sample":       fmt.Sprintf("Sample %d", i+1),
					"Penyakit":     "Tidak Diketahui",
					"Probabilitas": 0,
				})
			}
		}

		// Mengembalikan hasil sebagai JSON
		c.JSON(http.StatusOK, finalResults)
	})

	// Menjalankan server pada port 8080
	r.Run(":8080")
}
