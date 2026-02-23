package types

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2"
)

type StationInfo struct {
	NumPost    string
	CommonName string
	Lat        float64
	Lon        float64
	Alti       float64
}

type RainByStation struct {
	NumPost string
	Year    string
	Rain    float64
}

func InitDuckDB() (*sql.DB, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, err
	}
	return db, nil
}

func DownloadParquetFile(dpt string) error {
	parquetResources, err := fetchDataGouvDataset(dpt)
	if err != nil {
		return err
	}

	for _, resource := range parquetResources {
		out, err := os.Create(fmt.Sprintf("data/parquet/%s.parquet", resource.id))
		if err != nil {
			break
		}
		defer out.Close()
		resp, err := http.Get(resource.parquetUrl)
		if err != nil {
			break
		}
		_, err = io.Copy(out, resp.Body)
		fmt.Printf("File %s downloaded\n", resource.id)

	}

	return nil
}

type StationRain struct {
	NUM_POSTE string
	NOM_USUEL string
	AAAAMMJJ  string
	RR        float64
}

func GetRainByStationDuck(db *sql.DB, numPost string) ([]RainByStation, error) {
	stmt, err := db.Prepare(`
		SELECT NUM_POSTE, substr(CAST(AAAAMMJJ AS VARCHAR), 1, 4) as YEAR, sum(RR) as RAIN
		FROM read_parquet('data/parquet/*.parquet')
		WHERE CAST(NUM_POSTE AS VARCHAR) = ?
		GROUP BY NUM_POSTE, YEAR
		HAVING count(1) > 365 * 0.95
	`)

	if err != nil {
		return nil, err
	}

	rows, err := stmt.Query(numPost)

	if err != nil {
		return nil, err
	}

	response := make([]RainByStation, 0, 1000)

	var (
		numPoste, year string
		rain           float64
	)

	for {
		if rows.Next() {
			rows.Scan(&numPost, &year, &rain)
			response = append(response, RainByStation{
				NumPost: numPoste,
				Year:    year,
				Rain:    rain,
			})
		} else {
			break
		}
	}
	return response, nil
}

func GetStationRain(db *sql.DB, station string) []StationRain {
	stmt, err := db.Prepare("SELECT NUM_POSTE, NOM_USUEL, AAAAMMJJ, RR FROM read_parquet('data/parquet/*.parquet') WHERE CAST(NUM_POSTE AS VARCHAR) LIKE ?")
	if err != nil {
		return nil
	}
	rows, err := stmt.Query(station + "%")

	if err != nil {
		return nil
	}

	response := make([]StationRain, 0, 1000)

	var (
		numPoste, nomUsuelle, date string
		rr                         float64
	)

	for {
		if rows.Next() {
			rows.Scan(&numPoste, &nomUsuelle, &date, &rr)
			response = append(response, StationRain{
				NUM_POSTE: numPoste,
				NOM_USUEL: nomUsuelle,
				AAAAMMJJ:  date,
				RR:        rr,
			})
		} else {
			break
		}
	}
	return response
}

func GetStations(db *sql.DB) ([]StationInfo, error) {
	stmt, err := db.Prepare("SELECT DISTINCT NUM_POSTE, NOM_USUEL, LAT, LON, ALTI FROM read_parquet('data/parquet/*.parquet')")
	if err != nil {
		return nil, err
	}

	rows, err := stmt.Query()

	if err != nil {
		return nil, err
	}

	var numPoste, nomUsuel string
	var lat, long, alti float64

	response := make([]StationInfo, 0, 1000)

	for {
		if rows.Next() {
			rows.Scan(&numPoste, &nomUsuel, &lat, &long, &alti)
			response = append(response, StationInfo{
				NumPost:    numPoste,
				CommonName: nomUsuel,
				Lat:        lat,
				Lon:        long,
				Alti:       alti,
			})
		} else {
			break
		}
	}

	return response, nil
}

func GetClosestStationDuck(db *sql.DB, lat, long float64) (*StationInfo, error) {
	stmt, err := db.Prepare(`
		SELECT NUM_POSTE, NOM_USUEL, LON, LAT, ALTI, ((LAT - ?) * 111)*((LAT - ?)*111) + ((LON - ?)*111*COS((LON + ?) / 2))*((LON - ?)*111*COS((LON + ?) / 2)) as D
		FROM read_parquet('data/parquet/*.parquet') 
		WHERE D < 10*10
		ORDER BY D LIMIT 1
	`)

	if err != nil {
		return nil, err
	}

	var numPoste, nomUsuel string
	var latPoste, longPoste, alti, d float64

	rows := stmt.QueryRow(lat, lat, long, long, long, long)
	err = rows.Scan(&numPoste, &nomUsuel, &latPoste, &longPoste, &alti, &d)
	if err != nil {
		return nil, err
	}

	return &StationInfo{
		NumPost:    numPoste,
		CommonName: nomUsuel,
		Lat:        latPoste,
		Lon:        longPoste,
		Alti:       alti,
	}, nil

}

type DataGouvFile struct {
	id          string
	title       string
	description string
}

func readDataGouvFile() ([]DataGouvFile, error) {
	f, err := os.Open("./data/liens-datagouv-meteo.csv")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	csvReader := csv.NewReader(f)
	csvReader.Comma = ','

	// id,title,description,format,url,latest,filesize
	data := make([]DataGouvFile, 0, 100)

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		data = append(data, DataGouvFile{
			id:          record[0],
			title:       record[1],
			description: record[2],
		})
	}

	return data, nil
}

type DatasetResource struct {
	Description string            `json:"description"`
	Extras      map[string]string `json:"extras"`
	Id          string            `json:"id"`
}

type DataGouvDataset struct {
	Resources []DatasetResource `json:"resources"`
}

type WeatherResource struct {
	parquetUrl, id string
}

func fetchDataGouvDataset(dpt string) ([]WeatherResource, error) {
	rainDatasetId := "6569b51ae64326786e4e8e1a"
	url := fmt.Sprintf("https://www.data.gouv.fr/api/1/datasets/%s/", rainDatasetId)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	var dataset DataGouvDataset
	json.NewDecoder(resp.Body).Decode(&dataset)

	if len(dpt) == 1 {
		dpt = "0" + dpt
	}

	response := make([]WeatherResource, 0)

	for _, resource := range dataset.Resources {
		if strings.Contains(resource.Description, fmt.Sprintf("département %s", dpt)) {
			if (strings.Contains(resource.Description, "1950") || strings.Contains(resource.Description, "2022")) && !strings.Contains(resource.Description, "autres-parametres") {
				response = append(response, WeatherResource{
					parquetUrl: resource.Extras["analysis:parsing:parquet_url"],
					id:         resource.Id,
				})
			}
		}
	}

	return response, nil
}
