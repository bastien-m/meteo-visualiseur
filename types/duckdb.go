package types

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2"
)

func InitDuckDB(dpPath string) (*sql.DB, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, err
	}
	return db, nil
}

func DownloadParquetFile(dpt string) error {
	metadata, err := readDataGouvFile()
	if err != nil {
		return err
	}

	for _, m := range metadata {
		// file for this dpt
		if strings.Contains(m.title, fmt.Sprintf("_%s_", dpt)) {
			// we want 1950 to 2022 and 2022 to 2023 file (name doesnt change but they are up to date)
			if (strings.Contains(m.title, "1950") || strings.Contains(m.title, "2022")) && !strings.Contains(m.title, "autres-parametres") {
				parquetUrl := fmt.Sprintf("https://object.files.data.gouv.fr/hydra-parquet/hydra-parquet/%s.parquet", m.id)

				// check we dont already injected this file
				if len(dpt) == 1 {
					dpt = "0" + dpt
				}

				// stmt, err := db.Prepare("SELECT count(1) FROM file_processed WHERE department LIKE ?")
				// stmt.Exec(dpt)

				out, err := os.Create(fmt.Sprintf("data/%s.parquet", m.id))
				if err != nil {
					break
				}
				defer out.Close()
				resp, err := http.Get(parquetUrl)
				if err != nil {
					break
				}
				_, err = io.Copy(out, resp.Body)
				fmt.Printf("File %s downloaded\n", parquetUrl)
			}
		}
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
		SELECT NUM_POSTE, substr(AAAAMMJJ, 1, 4) as YEAR, sum(RR) as RAIN
		FROM read_parquet('data/*.parquet')
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
	stmt, err := db.Prepare("SELECT NUM_POSTE, NOM_USUEL, AAAAMMJJ, RR FROM read_parquet('data/*.parquet') WHERE CAST(NUM_POSTE AS VARCHAR) LIKE ?")
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

func GetStations(db *sql.DB) []StationInfo {
	stmt, err := db.Prepare("SELECT DISTINCT NUM_POSTE, NOM_USUEL, LAT, LONG, ALTI FROM read_parquet('data/*.parquet')")
	if err != nil {
		return nil
	}

	rows, err := stmt.Query()

	if err != nil {
		return nil
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

	return response
}

func GetClosestStationDuck(db *sql.DB, lat, long float64) *StationInfo {
	stmt, err := db.Prepare("SELECT num_post, common_name, long, lat FROM station_models ORDER BY (lat - ?)*(lat - ?) + (long - ?)*(long - ?) LIMIT 1")

	if err != nil {
		return nil
	}

	var numPoste, nomUsuel string
	var latPoste, longPoste, alti float64

	rows := stmt.QueryRow(lat, lat, long, long)
	err = rows.Scan(&numPoste, &nomUsuel, &lat, &long, &alti)
	if err != nil {
		return nil
	}

	return &StationInfo{
		NumPost:    numPoste,
		CommonName: nomUsuel,
		Lat:        latPoste,
		Lon:        longPoste,
		Alti:       alti,
	}

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
