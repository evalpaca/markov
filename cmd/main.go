package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/paka3m/markov"
)

func main() {
	const dbfile = "../../umi/cmd/markov_jumanpp3.db"
	//db, err := sql.Open("sqlite3", ":memory:")
	//os.Remove(dbfile)
	db, err := sql.Open("sqlite3", dbfile)
	//db.Exec(`pragma `)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	mc := markov.NewTalkService(db, "海未").ThinkingTime(time.Second * 5)
	for i := 0; i < 100; i++ {
		res := mc.TrigramMarkovChain().String()

		// fast textに頼るべきだろう。
		if len([]rune(res)) < 7 {
			fmt.Println("ng:", res)
			continue
		}
		fmt.Println("ok:", res)
	}
}
