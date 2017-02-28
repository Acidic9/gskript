package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fmt"

	"github.com/Acidic9/go-steam/steamapi"
	"github.com/Acidic9/sessions"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
)

func martiniSetup() *martini.ClassicMartini {
	r := martini.NewRouter()
	m := martini.New()
	m.Use(martini.Logger())
	m.Use(martini.Recovery())
	m.Use(martini.Static("www/assets"))
	m.Use(render.Renderer(render.Options{
		Directory:  "www/templates", // Specify what path to load the templates from.
		Extensions: []string{".tmpl", ".html"},
		Funcs:      []template.FuncMap{tmplFuncs}, // Specify helper function maps for templates to access.
	}))
	store := sessions.NewCookieStore([]byte("ewjkewh983uin3289"))
	m.Use(sessions.Sessions("u9309j3dj9j3d", store))
	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	return &martini.ClassicMartini{m, r}
}

func sessionSteamID(session sessions.Session) uint64 {
	steam64, valid := session.Get("steam64").Int64()
	if !valid {
		return 0
	}
	return uint64(steam64)
}

func parseParams(r *http.Request, session sessions.Session) map[string]interface{} {
	steam64, _ := session.Get("steam64").Int64()
	if steam64 != 0 {
		validateProfileInfo(db, uint64(steam64), r.RemoteAddr, session)
	}
	parse := make(map[string]interface{})
	parse["path"] = r.URL.Path
	parse["steam64"] = session.Get("steam64").Value
	parse["personaName"] = session.Get("personaName").Value
	parse["avatarFull"] = session.Get("avatarFull").Value
	parse["avatarMed"] = session.Get("avatarMed").Value
	parse["avatarSmall"] = session.Get("avatarSmall").Value
	parse["realName"] = session.Get("realName").Value
	return parse
}

func updateUser(db *sql.DB, steam64 uint64, ipAddr string, plySummary steamapi.PlayerSummaries) (sql.Result, error) {
	err := db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %v\n", err)
	}

	res, err := db.Exec(`INSERT INTO users (
		steamID,
		lastIP,
		firstLogin,
		lastLogin,
		communityVisibilityState,
		profileState,
		personaName,
		lastLogoff,
		profileURL,
		avatar,
		avatarMedium,
		avatarFull,
		personaState,
		realName,
		primaryClanID,
		timeCreated,
		personaStateFlags,
		locCountryCode
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE
			lastIP = ?,
			lastLogin = ?,
			communityVisibilityState = ?,
			profileState = ?,
			personaName = ?,
			lastLogoff = ?,
			profileURL = ?,
			avatar = ?,
			avatarMedium = ?,
			avatarFull = ?,
			personaState = ?,
			realName = ?,
			primaryClanID = ?,
			timeCreated = ?,
			personaStateFlags = ?,
			locCountryCode = ?`, steam64, ipAddr, time.Now(), time.Now(), plySummary.CommunityVisibilityState, plySummary.ProfileState, plySummary.PersonaName, time.Unix(int64(plySummary.LastLogOff), 0), plySummary.ProfileURL, plySummary.Avatar, plySummary.AvatarMedium, plySummary.AvatarFull, plySummary.PersonaState, plySummary.RealName, plySummary.PrimaryClanID, time.Unix(int64(plySummary.TimeCreated), 0), plySummary.PersonaStateFlags, plySummary.LocCountryCode, ipAddr, time.Now(), plySummary.CommunityVisibilityState, plySummary.ProfileState, plySummary.PersonaName, time.Unix(int64(plySummary.LastLogOff), 0), plySummary.ProfileURL, plySummary.Avatar, plySummary.AvatarMedium, plySummary.AvatarFull, plySummary.PersonaState, plySummary.RealName, plySummary.PrimaryClanID, time.Unix(int64(plySummary.TimeCreated), 0), plySummary.PersonaStateFlags, plySummary.LocCountryCode)
	if err != nil {
		return res, fmt.Errorf("failed to update %v profile info: %v\n", steam64, err)
	}

	return res, nil
}

func addScript(db *sql.DB, name, description string, price, discountPrice float64, steam64 uint64) (int, error) {
	err := db.Ping()
	if err != nil {
		return 0, fmt.Errorf("failed to ping database: %v\n", err)
	}

	var scriptID int
	err = db.QueryRow("SELECT IF(COUNT(id) >= 1, id, 0) FROM scripts ORDER BY id DESC LIMIT 1").Scan(&scriptID)
	if err != nil {
		return 0, fmt.Errorf("failed to find latest script: %v\n", err)
	}

	scriptID++

	_, err = db.Exec(`INSERT IGNORE INTO scripts (
		id,
		name,
		description,
		price,
		discountPrice,
		dateSubmitted,
		public,
		users_steamID
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		scriptID,
		name,
		description,
		price,
		discountPrice,
		time.Now(),
		true,
		steam64,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to add script to database: %v\n", err)
	}

	return scriptID, nil
}

func uuidToID(uuid string) int {
	var id string
	for _, part := range strings.Split(uuid, "-") {
		for _, by := range strings.Split(part, "") {
			id += strconv.FormatInt(int64([]rune(by)[0]), 10)
		}
	}
	if len(id) < 4 {
		return 0
	}
	convertedID, _ := strconv.ParseInt(id[:1]+id[1:2]+id[len(id)-2:len(id)-1]+id[len(id)-1:len(id)], 10, 64)
	return int(convertedID)
}

func round(x, unit float64) float64 {
	var rounded float64
	if x > 0 {
		rounded = float64(int64(x/unit+0.5)) * unit
	} else {
		rounded = float64(int64(x/unit-0.5)) * unit
	}
	formatted, err := strconv.ParseFloat(fmt.Sprintf("%.2f", rounded), 64)
	if err != nil {
		return rounded
	}
	return formatted
}

func validateProfileInfo(db *sql.DB, steam64 uint64, ipAddr string, session sessions.Session) {
	if steam64 == 0 {
		return
	}

	expires, exists := expiryTimes[steam64]

	if exists && time.Now().Sub(expires).Minutes() < config.Other.ProfileRefreshTime {
		return
	}

	updateProfileInfo(db, steam64, ipAddr, session)

	expiryTimes[steam64] = time.Now()
}

func updateProfileInfo(db *sql.DB, steam64 uint64, ipAddr string, session sessions.Session) {
	plySummary, err := steam.GetSinglePlayerSummaries(steam64)
	if err != nil {
		log.Printf("failed to get player summaries for %v: %v\n", steam64, err)
		return
	}

	session.Set("steam64", steam64)
	session.Set("personaName", plySummary.PersonaName)
	session.Set("avatarFull", plySummary.AvatarFull)
	session.Set("avatarMed", plySummary.AvatarMedium)
	session.Set("avatarSmall", plySummary.Avatar)
	session.Set("realName", plySummary.RealName)

	_, err = updateUser(db, steam64, ipAddr, plySummary)
	if err != nil {
		log.Println(err)
		return
	}
}
