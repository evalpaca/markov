package markov

import (
	"database/sql"
	"errors"
	"math"
	"strings"

	_ "github.com/mattn/go-sqlite3" //sqlite3
	"github.com/paka3m/jumangok/jmg"

	"fmt"
	"sort"
	"time"

	"github.com/jmcvetta/randutil"
	"github.com/k0kubun/pp"
)

const (
	bos = "<BOS>"
	eos = "<EOS>"
)

type Service struct {
	db    *sql.DB
	Chars []string
	Words []*jmg.Word
	Time  time.Duration
}

func NewTalkService(db *sql.DB, chars ...string) *Service {
	return &Service{
		db:    db,
		Chars: chars,
		Time:  time.Second * 10,
	}
}

func (s *Service) String() string {
	wc := len(s.Words)
	sl := make([]string, wc)
	for i := 0; i < wc; i++ {
		sl[i] = s.Words[i].Surface
	}
	r := strings.NewReplacer(
		bos, "</>",
		eos, "</>",
		"！", "！</>",
		"!", "！</>",
		"？", "？</>",
		"?", "？</>",
		"。", "。</>",
		"・・・", "...</>",
		"…", "...</>",
		"…", "...</>",
		"......", "...</>",
		"・", "</>",
		"「", "</>",
		"」", "</>",
		"\u3000", "</>",
		"\\", "",
		"&amp;", "&",
		"<数詞>", "315",
	)

	str := r.Replace(strings.Join(sl, ""))
	strs := strings.SplitN(str, "</>", -1)
	sort.Slice(strs, func(i, j int) bool {
		// 要研究:
		// rune化しないほうが、日本語が優先されやすい。改良の余地あり。
		// return len([]rune(strs[i])) > len([]rune(strs[j]))
		return len(strs[i]) > len(strs[j])
	})
	return strings.Replace(r.Replace(strs[0]), "</>", "", -1)
}

// tqb is a trigram query builder.
func tqb(t []*jmg.Word, mode int, chars ...string) (string, error) {
	tl := len(t)
	if tl < 2 {
		return "", errors.New("less than 2 words")
	}
	t1 := t[tl-1]
	t2 := t[tl-2]
	if t1 == nil {
		return "", errors.New("t1 is nil")
	}
	if t2 == nil {
		mode = -20
	}
	const n = 1000
	char := strings.Join(chars, "', '")

	var q string
	switch mode {
	case 0:
		q = fmt.Sprintf("SELECT w3, p3, cnt FROM trigram WHERE char in ('%s') AND w1  = '%s' AND w2 = '%s' AND p1 = '%s' AND p2 = '%s' ORDER BY cnt DESC limit %d", char, t2.Surface, t1.Surface, t2.Pos, t1.Pos, n)
	case 1:
		q = fmt.Sprintf("SELECT w3, p3, cnt FROM trigram WHERE char in ('%s') AND w1  = '%s' AND w2 = '%s' AND p2 = '%s' ORDER BY cnt DESC limit %d", char, t2.Surface, t1.Surface, t1.Pos, n)
	case 2:
		q = fmt.Sprintf("SELECT w3, p3, cnt FROM trigram WHERE char in ('%s') AND w1  = '%s' AND w2 = '%s' AND p2 like '%s' ORDER BY cnt DESC limit %d", char, t2.Surface, t1.Surface, strings.Split(t1.Pos, "<")[0]+"%%", n)
	case 3:
		q = fmt.Sprintf("SELECT w3, p3, cnt FROM trigram WHERE char in ('%s') AND w1  = '%s' AND w2 = '%s' ORDER BY cnt DESC limit %d", char, t2.Surface, t1.Surface, n)
	//case 2:
	//	q = fmt.Sprintf("SELECT w3, p3, cnt FROM trigram WHERE char = '%s' AND w2  = '%s' AND p2 = '%s' ORDER BY cnt DESC limit %d", char, t1.Surface, t1.Pos, n)
	//case 3:
	//q = fmt.Sprintf("SELECT w2, p2, cnt FROM trigram WHERE char = '%s' AND w1  = '%s' ORDER BY cnt DESC limit %d", char, t1.Surface, n)
	case -20:
		q = fmt.Sprintf("SELECT w2, p2, cnt FROM trigram WHERE char in ('%s') AND w1  = '%s' ORDER BY cnt DESC limit %d", char, t1.Surface, n)
	case 10:
		return "", errors.New("mode count reached max")
	}
	return q, nil
}

func (s *Service) ThinkingTime(t time.Duration) *Service {
	s.Time = t
	return s
}

func (s *Service) TrigramMarkovChain(words ...string) *Service {
	s.Words = make([]*jmg.Word, len(words)+3)
	s.Words[0] = &jmg.Word{Surface: "", Pos: "", Meta: &jmg.Meta{TFscore: 0}}
	s.Words[1] = &jmg.Word{Surface: "", Pos: "", Meta: &jmg.Meta{TFscore: 0}}
	s.Words[2] = &jmg.Word{Surface: bos, Lemma: bos, Pos: bos, Meta: &jmg.Meta{TFscore: 0}}
	mode := -20
	for i, w := range words {
		s.Words[i+3] = &jmg.Word{Surface: w, Pos: "", Meta: &jmg.Meta{TFscore: 0}}
	}
	qcmap := make(map[string][]randutil.Choice, 30)
	qm := make(map[string]int, 0)
	done := make(chan bool, 1)
	go func() {
		for {
			q, err := tqb(s.Words, mode, s.Chars...)
			if err != nil {
				break
			}
			if q == "" {
				mode++
				continue
			}
			// queryの回数をカウントするmap
			qm[q]++
			if qm[q] > 2 {
				mode++
				continue
			}
			cs, ok := qcmap[q]
			if !ok {
				rows, err := s.db.Query(q)
				if err != nil {
					fmt.Println(err)
					continue
				}
				//ポインタが先頭より前なので1つ目でも`Next()`が必要
				for rows.Next() {
					var w, p string
					var cnt int64
					err = rows.Scan(&w, &p, &cnt)
					if err != nil {
						fmt.Println(err)
					}
					switch p {
					case "名詞<数詞>", "特殊<記号>":
						cnt = int64(float64(cnt) / 2.0)
					}
					cs = append(cs, randutil.Choice{Weight: int(cnt), Item: &jmg.Word{Surface: w, Pos: p, Meta: &jmg.Meta{TFscore: cnt}}})
				}
				qcmap[q] = cs
			}

			switch len(cs) {
			default:
				mode = 0
			case 0:
				mode++
				continue
			case 1:
				tri, ok := cs[0].Item.(*jmg.Word)
				if ok && tri != nil {
					s.Words = append(s.Words, tri)
					mode = 0
					continue
				}
				mode++
				continue
			}

		choose_loop:
			for i := 0; i < len(cs); i++ {
				result, _ := randutil.WeightedChoice(cs)
				tri, ok := result.Item.(*jmg.Word)
				switch {
				case !ok:
					continue
				case tri == nil, tri.Surface == eos:
					done <- true
					break choose_loop
				case strings.HasPrefix(tri.Surface, ">"), strings.HasPrefix(tri.Surface, "\""):
					continue
				// case tri.Pos == "名詞<その他>":
				// 	continue
				case !func() bool { //重複検査
					l := len(s.Words)
					ll := math.Min(float64(l-1), 6.0)
					for j := 1; float64(j) < ll; j++ {
						kk := s.Words[l-j]
						if kk == nil {
							pp.Println("debug]kkerr", l, l-j)
							return false
						}
						if tri.Surface == kk.Surface {
							return true
						}
					}
					return false
				}():
					s.Words = append(s.Words, tri)
					break choose_loop
				}
			}

		}
	}()

	select {
	case <-done:
		return s
	case <-time.After(s.Time):
		return s
	}

}
