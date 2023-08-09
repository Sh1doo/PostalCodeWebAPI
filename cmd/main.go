package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"

	_ "github.com/go-sql-driver/mysql"
)

type Response struct {
	Postal_code        string  `json:"postal_code"`
	Hit_count          int     `json:"hit_count"`
	Address            string  `json:"address"`
	Tokyo_sta_distance float64 `json:"tokyo_sta_distance"`
}

type GeoApiLocation struct {
	City       string `json:"city"`
	City_Kana  string `json:"city_kana"`
	Town       string `json:"town"`
	Town_Kana  string `json:"town_kana"`
	X          string `json:"x"`
	Y          string `json:"y"`
	Prefecture string `json:"prefecture"`
	Postal     string `json:"postal"`
}

type GeoApiResponse struct {
	Location []GeoApiLocation `json:"location"`
}

type GeoApiResponseJson struct {
	Response GeoApiResponse `json:"response"`
}

type Accesslog struct {
	Postal_code   string `json:"postal_code"`
	Request_count int    `json:"request_count"`
}

type Accesslogs struct {
	Access_logs []Accesslog `json:"access_logs"`
}

func Min(x, y int) int {
	if x < y {
		return x
	} else {
		return y
	}
}

func FindCommonStrings(str1, str2 []byte) []byte {
	var common_len int

	common_len = len(str1)
	for i := 0; i < Min(len(str1), len(str2)); i++ {
		if str1[i] != str2[i] {
			common_len = i - 1
			break
		}
	}

	if common_len == -1 {
		return []byte("NO COMMON STRING")
	} else {
		return str1[0:common_len]
	}
}

func calcTokyoStaDistance(x, y float64) float64 {
	xt := 139.7673068
	yt := 35.6809591
	R := 6371

	var distance float64 = math.Pi * float64(R) / 180 * math.Sqrt(math.Pow((x-xt)*math.Cos(math.Pi*(y+yt)/360), 2)+math.Pow(y-yt, 2))

	return distance
}

func main() {
	http.HandleFunc("/", Handler_Root)
	http.HandleFunc("/address", Handler_Postal)
	http.HandleFunc("/address/access_logs", Handler_AccessLogs)
	http.ListenAndServe(":8080", nil)
}

func Handler_Root(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Response")
}

func Handler_Postal(w http.ResponseWriter, r *http.Request) {
	//リクエストパラメータで与えた郵便番号
	postal_code := r.FormValue("postal_code")

	url_values := url.Values{"method": {"searchByPostal"}, "postal": {postal_code}}
	geoapi_get_response, err := http.Get("https://geoapi.heartrails.com/api/json" + "?" + url_values.Encode())
	if err != nil {
		fmt.Fprintf(w, "error : geoapi get method failed.")
	}
	defer geoapi_get_response.Body.Close()

	geoapi_reponse_bodyBytes, err := io.ReadAll(geoapi_get_response.Body)
	if err != nil {
		fmt.Fprintf(w, "error : reponse body read failed.")
	}

	var geoapi_response GeoApiResponseJson
	err = json.Unmarshal(geoapi_reponse_bodyBytes, &geoapi_response)
	if err != nil {
		fmt.Fprintf(w, "error : Unmarshal failed.")
	}

	//該当する地域の数
	hit_count := len(geoapi_response.Response.Location)
	if hit_count == 0 {
		fmt.Fprintf(w, "Not Exists PostalCode")
	}

	//共通する部分の住所
	common_address := []byte(fmt.Sprintf("%s%s%s", geoapi_response.Response.Location[0].Prefecture, geoapi_response.Response.Location[0].City, geoapi_response.Response.Location[0].Town))
	for i := 1; i < hit_count; i++ {
		target_location := geoapi_response.Response.Location[i]
		address := []byte(fmt.Sprintf("%s%s%s", target_location.Prefecture, target_location.City, target_location.Town))
		common_address = FindCommonStrings(common_address, address)
	}

	//東京駅から最も離れた地域から東京駅までの距離
	var longest_tokyo_sta_distance float64
	longest_tokyo_sta_distance = 0

	for i := 0; i < hit_count; i++ {
		target_location_x, _ := strconv.ParseFloat(geoapi_response.Response.Location[i].X, 64)
		target_location_y, _ := strconv.ParseFloat(geoapi_response.Response.Location[i].Y, 64)
		longest_tokyo_sta_distance = math.Max(longest_tokyo_sta_distance, calcTokyoStaDistance(target_location_x, target_location_y))
	}

	var responseJson Response
	responseJson.Postal_code = postal_code
	responseJson.Hit_count = hit_count
	responseJson.Address = string(common_address)
	responseJson.Tokyo_sta_distance = math.Round(longest_tokyo_sta_distance*10) / 10

	response, err := json.Marshal(responseJson)
	if err != nil {
		fmt.Fprintf(w, "error : encode to json failed")
	}

	w.Write(response)

	dsn := "testuser:password@tcp(dockerMySQL:3306)/testdatabase"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintf(w, "error : connect database failed")
	}
	defer db.Close()

	_, err = db.Exec("INSERT INTO access_logs (postal_code) VALUES (?);", postal_code)
	if err != nil {
		fmt.Fprintf(w, "insert query failed: %v", err)
	}
}

func Handler_AccessLogs(w http.ResponseWriter, r *http.Request) {
	dsn := "testuser:password@tcp(dockerMySQL:3306)/testdatabase"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintf(w, "error : connect database failed")
	}
	defer db.Close()

	result, err := db.Query("SELECT DISTINCT postal_code FROM access_logs;")
	if err != nil {
		fmt.Fprintf(w, "error : select query failed")
	}
	defer result.Close()

	var access_logs Accesslogs
	for result.Next() {
		var access_log Accesslog
		access_log.Request_count = 0
		err := result.Scan(&access_log.Postal_code)
		if err != nil {
			fmt.Fprintf(w, "error : scan failed")
		}
		access_logs.Access_logs = append(access_logs.Access_logs, access_log)
	}

	access_count := map[string]int{}
	result, err = db.Query("SELECT ALL postal_code FROM access_logs;")
	if err != nil {
		fmt.Fprintf(w, "error : select query failed")
	}

	for result.Next() {
		var scaned_postal string
		err = result.Scan(&scaned_postal)
		if err != nil {
			fmt.Fprintf(w, "error : scan failed where select all")
		}
		if _, ok := access_count[scaned_postal]; !ok {
			access_count[scaned_postal] = 0
		}
		access_count[scaned_postal] += 1
	}

	for i := 0; i < len(access_logs.Access_logs); i++ {
		access_logs.Access_logs[i].Request_count = access_count[access_logs.Access_logs[i].Postal_code]
	}

	response, err := json.Marshal(access_logs)
	if err != nil {
		fmt.Fprintf(w, "error : encode to json failed")
	}
	w.Write(response)

}
