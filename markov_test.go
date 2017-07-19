package markov

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestMc(t *testing.T) {
	var dbfile = "./markov_jumanpp.db"
	//db, err := sql.Open("sqlite3", ":memory:")
	//os.Remove(dbfile)
	db, err := sql.Open("sqlite3", dbfile)
	//db.Exec(`pragma `)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	mc := NewTalkService(db, "海未").ThinkingTime(time.Second * 20)
	for i := 0; i < 1; i++ {
		fmt.Println(mc.TrigramMarkovChain().String())
	}
}
