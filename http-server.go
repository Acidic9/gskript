package main

import (
	"database/sql"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"gopkg.in/mgo.v2/bson"

	_ "github.com/go-sql-driver/mysql"

	"fmt"

	"sync"

	"strings"

	"github.com/Acidic9/go-steam/steamapi"
	"github.com/Acidic9/sessions"
	"github.com/Acidic9/utils"
	"github.com/BurntSushi/toml"
	"github.com/ararog/timeago"
	"github.com/disintegration/imaging"
	"github.com/go-martini/martini"
	"github.com/guregu/null"
	"github.com/martini-contrib/render"
	"github.com/solovev/steam_go"
)

type user struct {
	ID bson.ObjectId `json:"id" bson:"_id,omitempty"`
	steamapi.PlayerSummaries
	LastIP    string `bson:"omitempty"`
	LastLogin time.Time
}

type script struct {
	ID              bson.ObjectId `json:"id" bson:"_id,omitempty"`
	Name            string
	Description     string
	Price           float64
	DiscountPrice   float64
	DateSubmitted   time.Time
	Steam64         string
	Ratings         []bool
	Public          bool
	DisplayName     string
	AvatarURLMedium string
}

var (
	db            *sql.DB
	steam         steamapi.Key
	scriptCounter sync.Locker
	expiryTimes   = make(map[uint64]time.Time)
)

var config struct {
	Database struct {
		Host   string
		Pass   string
		User   string
		DBName string
	}
	Steam struct {
		APIKey string
	}
	Other struct {
		ProfileRefreshTime float64
	}
}

func init() {
	logFileName := "logs/" + strconv.FormatInt(utils.MakeTimestamp(), 10) + ".log"
	logFile, err := os.OpenFile(logFileName, os.O_WRONLY|os.O_CREATE, 0640)
	if err != nil {
		log.Fatalln(err)
	}

	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetFlags(log.Lshortfile)
	log.SetOutput(mw)

	_, err = toml.DecodeFile("config.toml", &config)
	if err != nil {
		log.Fatalf("unable to read config.toml: %v\n", err)
	}

	steam = steamapi.NewKey(config.Steam.APIKey)

	dbConnStr := fmt.Sprintf(
		"%v:%v@%v/%v?parseTime=true",
		config.Database.User,
		config.Database.Pass,
		config.Database.Host,
		config.Database.DBName,
	)
	db, err = sql.Open("mysql", dbConnStr)
	if err != nil {
		log.Fatalf("unable to connect to '%v' with user '%v': %v\n", config.Database.Host, config.Database.User, err)
	}
}

func main() {
	m := martiniSetup()

	m.NotFound(func(r *http.Request, ren render.Render, session sessions.Session) {
		parse := parseParams(r, session)
		ren.HTML(http.StatusNotFound, "404", parse)
	})

	m.Get("/login", func(w http.ResponseWriter, r *http.Request, ren render.Render, session sessions.Session) {
		opID := steam_auth.NewOpenId(r)
		switch opID.Mode() {
		case "":
			http.Redirect(w, r, opID.AuthUrl(), http.StatusSeeOther)
		case "cancel":
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		default:
			steam64Str, err := opID.ValidateAndGetId()
			if err != nil {
				ren.HTML(http.StatusOK, "login-failed", nil)
				log.Printf("failed to login: %v\n", err)
				return
			}

			steam64, err := strconv.ParseUint(steam64Str, 10, 64)
			if err != nil {
				ren.HTML(http.StatusOK, "login-failed", nil)
				log.Printf("failed to parse steam64str=%v to uint64: %v\n", steam64Str, err)
				return
			}

			updateProfileInfo(db, steam64, r.RemoteAddr, session)

			returnPath := r.URL.Query().Get("return")
			if returnPath == "" {
				returnPath = "/"
			}
			http.Redirect(w, r, returnPath, http.StatusSeeOther)
		}
	})

	m.Get("/logout", func(w http.ResponseWriter, r *http.Request, session sessions.Session) {
		session.Clear()

		returnPath := r.URL.Query().Get("return")
		if returnPath == "" {
			returnPath = "/"
		}

		http.Redirect(w, r, returnPath, http.StatusSeeOther)
	})

	m.Get(`/profile/(?P<steam64str>\d{17})`, func(w http.ResponseWriter, r *http.Request, ren render.Render, params martini.Params, session sessions.Session) {
		parse := parseParams(r, session)

		steam64, _ := strconv.ParseUint(params["steam64str"], 10, 64)

		parse["profileSteam64"] = steam64

		var (
			personaName    sql.NullString
			realName       sql.NullString
			biography      sql.NullString
			email          sql.NullString
			website        sql.NullString
			lastLogin      null.Time
			avatarFull     sql.NullString
			locCountryCode sql.NullString
			firstLogin     null.Time
		)

		err := db.Ping()
		if err != nil {
			log.Printf("failed to ping database: %v\n", err)
		}

		row := db.QueryRow("SELECT personaName, realName, biography, email, website, lastLogin, avatarFull, locCountryCode, firstLogin FROM users WHERE steamID=? LIMIT 1", steam64)
		err = row.Scan(
			&personaName,
			&realName,
			&biography,
			&email,
			&website,
			&lastLogin,
			&avatarFull,
			&locCountryCode,
			&firstLogin,
		)
		if err != nil {
			parse["profileSteam64"] = 0
			log.Printf("could not find user `%v`: %v\n", steam64, err)
		}

		parse["profilePersonaName"] = personaName.String
		parse["profileRealName"] = realName.String
		parse["profileBiography"] = biography.String
		parse["profileEmail"] = email.String
		parse["profileWebsite"] = website.String
		parse["profileLastLogin"], _ = timeago.TimeAgoFromNowWithTime(lastLogin.Time)
		parse["profileAvatarFull"] = avatarFull.String
		parse["profileLocCountryCode"] = locCountryCode.String
		parse["profileFirstLogin"] = firstLogin.Time.Format("2 Jan, 2006")
		parse["ownProfile"] = false
		if sessionSteamID(session) == uint64(steam64) {
			parse["ownProfile"] = true
		}
		if steam64 == 76561198132612090 {
			parse["isOwner"] = true
		}

		ren.HTML(http.StatusOK, "profile", parse)
	})

	m.Get(`/((scripts(\/(?P<pageNum>\d+)?)?)?)`, func(w http.ResponseWriter, r *http.Request, ren render.Render, params martini.Params, session sessions.Session) {
		parse := parseParams(r, session)

		pageNum, _ := strconv.Atoi(params["pageNum"])
		if pageNum < 1 {
			pageNum = 1
		}

		parse["pageNum"] = strconv.Itoa(pageNum)
		parse["isHomePage"] = true
		parse["scripts"] = make(map[string]*script)
		parse["sortBy"] = 0
		parse["searchText"] = r.URL.Query().Get("search")

		sortBy := r.URL.Query().Get("sort")

		var sortQuery string
		searchQuery := "%" + r.URL.Query().Get("search") + "%"

		switch sortBy {
		case "cheap":
			sortQuery = "discountPrice ASC"
			parse["sortBy"] = 1
		case "expensive":
			sortQuery = "discountPrice DESC"
			parse["sortBy"] = 2
		case "ratings":
			sortQuery = "(positiveRatings / (positiveRatings + negativeRatings)) DESC"
			parse["sortBy"] = 3
		default:
			sortQuery = "DateSubmitted DESC"
			parse["sortBy"] = 0
		}

		scripts := make([]struct {
			ID              int64
			Name            string
			Description     string
			Price           string
			DiscountPrice   string
			PositiveRatings int64
			NegativeRatings int64
			DateSubmitted   time.Time
			SteamID         string
			PersonaName     string
			AvatarMedium    string
			TotalVotes      int64
			Link            string
			Stars           []int
		}, 0, 10)

		rows, err := db.Query("SELECT id, name, REPLACE(REPLACE(description, '\r', ''), '\n', ''), price, discountPrice, positiveRatings, negativeRatings, dateSubmitted, steamID, personaName, avatarMedium FROM scripts, users WHERE public=TRUE and scripts.users_steamID=users.steamID and name LIKE ? ORDER BY "+sortQuery+", dateSubmitted DESC LIMIT ?, ?",
			searchQuery,
			(pageNum*9)-9,
			((pageNum*9)-9)+9,
		)
		if err != nil {
			log.Printf("unable to query scripts from database: %v\n", err)
		} else {
			for rows.Next() {
				var (
					id              sql.NullInt64
					name            sql.NullString
					description     sql.NullString
					price           sql.NullFloat64
					discountPrice   sql.NullFloat64
					positiveRatings sql.NullInt64
					negativeRatings sql.NullInt64
					dateSubmitted   null.Time
					steamID         sql.NullString
					personaName     sql.NullString
					avatarMedium    sql.NullString
				)

				err = rows.Scan(&id, &name, &description, &price, &discountPrice, &positiveRatings, &negativeRatings, &dateSubmitted, &steamID, &personaName, &avatarMedium)
				if err != nil {
					log.Printf("unable to scan database row: %v\n", err)
					continue
				}

				stars := make([]int, 5) // 0 = no star, 1 = full star, 2 = half star

				if (positiveRatings.Int64 + negativeRatings.Int64) > 0 {
					votePercent := round(float64(float64(float64(positiveRatings.Int64)/float64(positiveRatings.Int64+negativeRatings.Int64))/2), .05)
					if votePercent == 0.05 {
						stars[0] = 2
					} else if votePercent >= 0.1 {
						stars[0] = 1
					}
					if votePercent == 0.15 {
						stars[1] = 2
					} else if votePercent >= 0.2 {
						stars[1] = 1
					}
					if votePercent == 0.25 {
						stars[2] = 2
					} else if votePercent >= 0.3 {
						stars[2] = 1
					}
					if votePercent == 0.35 {
						stars[3] = 2
					} else if votePercent >= 0.4 {
						stars[3] = 1
					}
					if votePercent == 0.45 {
						stars[4] = 2
					} else if votePercent >= 0.5 {
						stars[4] = 1
					}
				}

				scripts = append(scripts, struct {
					ID              int64
					Name            string
					Description     string
					Price           string
					DiscountPrice   string
					PositiveRatings int64
					NegativeRatings int64
					DateSubmitted   time.Time
					SteamID         string
					PersonaName     string
					AvatarMedium    string
					TotalVotes      int64
					Link            string
					Stars           []int
				}{
					id.Int64,
					name.String,
					description.String,
					fmt.Sprintf("%.2f", price.Float64),
					fmt.Sprintf("%.2f", discountPrice.Float64),
					positiveRatings.Int64,
					negativeRatings.Int64,
					dateSubmitted.Time,
					steamID.String,
					personaName.String,
					avatarMedium.String,
					positiveRatings.Int64 + negativeRatings.Int64,
					fmt.Sprintf("/%v/%v", id.Int64, strings.Replace(strings.ToLower(name.String), " ", "-", -1)),
					stars,
				})
			}

			rows.Close()
		}

		parse["scripts"] = scripts

		var count int
		err = db.QueryRow("SELECT COUNT(id) FROM scripts WHERE name LIKE ?", searchQuery).Scan(&count)
		if err != nil {
			log.Printf("unable to count scripts from database: %v\n", err)
		}

		pageCount := int(math.Ceil(float64(count) / float64(10)))

		if pageNum > pageCount {
			parse["error"] = "Page not found"
		}

		pageList := make([][]string, 0, 11)

		if pageNum <= 2 {
			pageList = append(pageList, []string{"", "<"})
		} else {
			pageList = append(pageList, []string{strconv.Itoa(pageNum - 1), "<"})
		}

		pageList = append(pageList, []string{"1", "1"})
		if pageCount >= 2 {
			pageList = append(pageList, []string{"2", "2"})
		}
		if pageCount >= 3 {
			pageList = append(pageList, []string{"3", "3"})
		}

		if pageNum > 5 {
			pageList = append(pageList, []string{"", "..."})
		}

		if pageNum-1 > 3 && pageNum-1 < pageCount-2 {
			pageList = append(pageList, []string{strconv.Itoa(pageNum - 1), strconv.Itoa(pageNum - 1)})
		}
		if pageNum > 3 && pageNum < pageCount-2 {
			pageList = append(pageList, []string{strconv.Itoa(pageNum), strconv.Itoa(pageNum)})
		}
		if pageNum+1 > 3 && pageNum+1 < pageCount-2 {
			pageList = append(pageList, []string{strconv.Itoa(pageNum + 1), strconv.Itoa(pageNum + 1)})
		}

		if pageCount >= pageNum+5 {
			pageList = append(pageList, []string{"", "..."})
		}

		if pageCount > 3 {
			pageList = append(pageList, []string{strconv.Itoa(pageCount - 2), strconv.Itoa(pageCount - 2)})
		}
		if pageCount > 3 {
			pageList = append(pageList, []string{strconv.Itoa(pageCount - 1), strconv.Itoa(pageCount - 1)})
		}
		if pageCount > 3 {
			pageList = append(pageList, []string{strconv.Itoa(pageCount), strconv.Itoa(pageCount)})
		}

		if pageNum > pageCount-1 {
			pageList = append(pageList, []string{strconv.Itoa(pageNum), ">"})
		} else {
			pageList = append(pageList, []string{strconv.Itoa(pageNum + 1), ">"})
		}

		var filters string
		if len(r.URL.Query().Get("search")) > 0 || len(r.URL.Query().Get("sort")) > 0 {
			filters = "?"
		}
		if len(r.URL.Query().Get("search")) > 0 {
			filters += "search=" + r.URL.Query().Get("search")
			if len(r.URL.Query().Get("sort")) > 0 {
				filters += "&"
			}
		}
		if len(r.URL.Query().Get("sort")) > 0 {
			filters += "sort=" + r.URL.Query().Get("sort")
		}

		parse["pageList"] = pageList
		parse["filters"] = filters

		ren.HTML(http.StatusOK, "scripts", parse)
	})

	m.Get("/upload", func(w http.ResponseWriter, r *http.Request, ren render.Render, params martini.Params, session sessions.Session) {
		parse := parseParams(r, session)
		ren.HTML(http.StatusOK, "upload", parse)
	})

	m.Post("/do/upload-script", func(w http.ResponseWriter, r *http.Request, ren render.Render, params martini.Params, session sessions.Session) {
		var resp struct {
			Success bool   `json:"success"`
			Err     string `json:"err"`
		}

		steam64 := sessionSteamID(session)
		if steam64 == 0 {
			resp.Success = false
			resp.Err = "You must be logged in to upload a script."
			ren.JSON(http.StatusOK, resp)
			return
		}

		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong when uploading your addon. Contact us if this issue continues to occur."
			ren.JSON(http.StatusOK, resp)
			log.Printf("could not ParseMultipartForm: %v\n", err)
			return
		}

		scriptName := r.FormValue("script-name")
		scriptDescription := r.FormValue("script-description")
		scriptPrice, err := strconv.ParseFloat(r.FormValue("script-price"), 64)
		if err != nil {
			resp.Success = false
			resp.Err = "Script price is invalid."
			ren.JSON(http.StatusOK, resp)
			return
		}
		scriptDiscountPrice, err := strconv.ParseFloat(r.FormValue("script-discount"), 64)
		if err != nil {
			resp.Success = false
			resp.Err = "Script discount price is invalid."
			ren.JSON(http.StatusOK, resp)
			return
		}

		if len(scriptName) < 4 {
			resp.Success = false
			resp.Err = "Script name must be 4 or more characters long."
			ren.JSON(http.StatusOK, resp)
			return
		}
		if len(scriptDescription) < 80 {
			resp.Success = false
			resp.Err = "Script description must be more 80 or more characters long."
			ren.JSON(http.StatusOK, resp)
			return
		}
		if scriptPrice < 1 {
			resp.Success = false
			resp.Err = "Script price must be more than $1."
			ren.JSON(http.StatusOK, resp)
			return
		}
		if scriptDiscountPrice < 1 {
			resp.Success = false
			resp.Err = "Script discount must be more than $1."
			ren.JSON(http.StatusOK, resp)
			return
		}

		bannerImg, bannerHeader, err := r.FormFile("script-banner")
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong when uploading your script banner."
			ren.JSON(http.StatusOK, resp)
			log.Printf("could not read scriptBanner: %v\n", err)
			return
		}
		defer bannerImg.Close()

		zipFile, zipHeader, err := r.FormFile("script-zip")
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong when uploading your script ZIP file."
			ren.JSON(http.StatusOK, resp)
			log.Printf("could not read scriptZip: %v\n", err)
			return
		}
		defer zipFile.Close()

		if bannerHeader.Filename == "" {
			resp.Success = false
			resp.Err = "Invalid script banner."
			ren.JSON(http.StatusOK, resp)
			return
		}
		if zipHeader.Filename == "" {
			resp.Success = false
			resp.Err = "Invalid script ZIP file."
			ren.JSON(http.StatusOK, resp)
			return
		}

		scriptID, err := addScript(db, scriptName, scriptDescription, scriptPrice, scriptDiscountPrice, steam64)
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong and the process was incomplete."
			ren.JSON(http.StatusOK, resp)
			log.Println(err)
			return
		}

		scriptIDStr := strconv.Itoa(scriptID)

		os.Mkdir("www/assets/img/scripts/"+scriptIDStr, 644)

		bannerPath := "www/assets/img/scripts/" + scriptIDStr + "/banner.jpg"
		bannerSmallPath := "www/assets/img/scripts/" + scriptIDStr + "/banner_small.jpg"
		zipPath := "www/scripts/" + scriptIDStr + ".zip"

		bannerOut, err := os.Create(bannerPath)
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong and we could not complete the process."
			ren.JSON(http.StatusOK, resp)
			log.Printf("could not create file '%v': %v\n", bannerPath, err)
			return
		}
		defer bannerOut.Close()

		scriptOut, err := os.Create(zipPath)
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong and we could not complete the process."
			ren.JSON(http.StatusOK, resp)
			log.Printf("could not create file '%v': %v\n", zipPath, err)
			return
		}
		defer scriptOut.Close()

		_, err = io.Copy(bannerOut, bannerImg)
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong and we could not complete the process."
			ren.JSON(http.StatusOK, resp)
			log.Printf("could not copy to file '%v': %v\n", bannerPath, err)
			return
		}

		bannerFile, err := os.Open(bannerPath)
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong and we could not complete the process."
			ren.JSON(http.StatusOK, resp)
			log.Printf("could not open file '%v': %v\n", bannerPath, err)
			return
		}
		defer bannerFile.Close()

		bannerSmall, err := os.Create(bannerSmallPath)
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong and we could not complete the process."
			ren.JSON(http.StatusOK, resp)
			log.Printf("could not create file '%v': %v\n", bannerSmallPath, err)
			return
		}
		defer bannerSmall.Close()

		_, err = io.Copy(bannerSmall, bannerFile)
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong and we could not complete the process."
			ren.JSON(http.StatusOK, resp)
			log.Printf("could not copy to file '%v': %v\n", bannerSmallPath, err)
			return
		}

		banner, err := imaging.Open(bannerPath)
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong and we could not complete the process."
			ren.JSON(http.StatusOK, resp)
			log.Printf("could not open file '%v': %v\n", bannerPath, err)
			return
		}

		resized := imaging.Resize(banner, 1024, 256, imaging.Lanczos)
		err = imaging.Save(resized, bannerPath)
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong and we could not complete the process."
			ren.JSON(http.StatusOK, resp)
			log.Printf("could not resize image '%v': %v\n", bannerPath, err)
			return
		}

		bannerImgSmall, err := imaging.Open(bannerSmallPath)
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong and we could not complete the process."
			ren.JSON(http.StatusOK, resp)
			log.Printf("could not open file '%v': %v\n", bannerSmallPath, err)
			return
		}

		resized = imaging.Resize(bannerImgSmall, 384, 96, imaging.Lanczos)
		err = imaging.Save(resized, bannerSmallPath)
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong and we could not complete the process."
			ren.JSON(http.StatusOK, resp)
			log.Printf("could not resize image '%v': %v\n", bannerSmallPath, err)
			return
		}

		_, err = io.Copy(scriptOut, zipFile)
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong and we could not complete the process."
			ren.JSON(http.StatusOK, resp)
			log.Printf("could not copy to file '%v': %v\n", zipFile, err)
			return
		}

		resp.Success = true
		resp.Err = ""
		ren.JSON(http.StatusOK, resp)
	})

	m.Post("/do/update-profile/bio", func(w http.ResponseWriter, r *http.Request, ren render.Render, params martini.Params, session sessions.Session) {
		var resp struct {
			Success bool   `json:"success"`
			Err     string `json:"err"`
		}

		steam64 := sessionSteamID(session)
		if steam64 == 0 {
			resp.Success = false
			resp.Err = "You must be logged in to perform this action."
			ren.JSON(http.StatusOK, resp)
			return
		}

		err := r.ParseForm()
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong when updating your bio."
			ren.JSON(http.StatusOK, resp)
			return
		}

		var (
			bio = strings.TrimFunc(r.FormValue("bio"), func(r rune) bool {
				if r == ' ' || r == '\n' {
					return true
				}
				return false
			})
		)

		err = db.Ping()
		if err != nil {
			log.Printf("failed to ping database: %v\n", err)
		}

		_, err = db.Exec("UPDATE users SET biography=? WHERE steamID=?",
			bio,
			steam64,
		)
		if err != nil {
			resp.Success = false
			resp.Err = "Something went wrong and your profile bio was not updated."
			ren.JSON(http.StatusOK, resp)
			log.Printf("failed to update profile '%v' with values biography=%v in database: %v\n", steam64, bio, err)
			return
		}

		resp.Success = true
		resp.Err = ""
		ren.JSON(http.StatusOK, resp)
	})

	m.Run()
}
