package file

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

import (
	"github.com/timtadh/cc-survey/clones"
	"github.com/timtadh/cc-survey/models"
	"github.com/timtadh/data-structures/set"
	"github.com/timtadh/data-structures/types"
)


type SurveyLogStore struct {
	questions []models.Renderable
	clones []*clones.Clone
	cloneIdxs *set.SortedSet
	answersPath string
	cache *surveyCache
	lock sync.Mutex
}

type surveyCache struct {
	survey *models.Survey
	answerCount int
}

func NewSurveyStore(dir string, questions []models.Renderable, clones []*clones.Clone) (*SurveyLogStore, error) {
	fi, err := os.Stat(dir)
	if err != nil && os.IsNotExist(err) {
		err := os.Mkdir(dir, 0775)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("%v is not a directory", dir)
	}
	cloneIdxs := set.NewSortedSet(len(clones))
	for i := 0; i < len(clones); i++ {
		cloneIdxs.Add(types.Int(i))
	}
	qPath := filepath.Join(dir, "questions")
	if q, err := os.Create(qPath); err != nil {
		return nil, err
	} else {
		defer q.Close()
		bytes, err := json.Marshal(questions)
		if err != nil {
			return nil, err
		}
		_, err = q.Write(bytes)
		if err != nil {
			return nil, err
		}
	}
	st := &SurveyLogStore{
		questions: questions,
		clones: clones,
		cloneIdxs: cloneIdxs,
		answersPath: filepath.Join(dir, "answers"),
	}
	return st, nil
}

func (st *SurveyLogStore) Do(f func(*models.Survey) error) error {
	st.lock.Lock()
	defer st.lock.Unlock()
	answersCount, s, err := st.load()
	if err != nil {
		return err
	}
	err = f(s)
	if err != nil {
		return err
	}
	return st.save(answersCount, s)
}

func (st *SurveyLogStore) load() (int, *models.Survey, error) {
	if st.cache != nil {
		return st.cache.answerCount, st.cache.survey, nil
	}
	answers := make([]*models.SurveyAnswer, 0, len(st.clones)*2)
	answered := set.NewSortedSet(len(st.clones))
	err := createOrOpen(st.answersPath,
		func(path string) (err error) {
			// create file
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			return f.Close()
		},
		func(path string) (err error) {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			return st.loadFile(f, &answers, answered)
		},
	)
	if err != nil {
		return 0, nil, err
	}
	unanswered := st.cloneIdxs.Subtract(answered)
	s := &models.Survey{
		Questions: st.questions,
		Clones: st.clones,
		Unanswered: unanswered,
		Answers: answers,
	}
	st.cache = &surveyCache{
		survey: s,
		answerCount: len(answers),
	}
	return len(answers), s, nil
}

func (st *SurveyLogStore) loadFile(f io.Reader, answers *[]*models.SurveyAnswer, answered *set.SortedSet) error {
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		err := st.loadLine(line, answers, answered)
		if err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (st *SurveyLogStore) loadLine(line []byte, answers *[]*models.SurveyAnswer, answered *set.SortedSet) error {
	var a models.SurveyAnswer
	err := json.Unmarshal(line, &a)
	if err != nil {
		return err
	}
	answered.Add(types.Int(a.CloneID))
	*answers = append(*answers, &a)
	return nil
}

func (st *SurveyLogStore) save(answersCount int, s *models.Survey) error {
	f, err := os.OpenFile(st.answersPath, os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0660)
	if err != nil && os.IsNotExist(err) {
		f, err = os.OpenFile(st.answersPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_SYNC, 0660)
		if err != nil {
			return err
		}
		answersCount = 0
	} else if err != nil {
		return err
	}
	defer f.Close()
	err = st.saveFile(f, answersCount, s)
	if err != nil {
		return err
	}
	st.cache = &surveyCache{
		survey: s,
		answerCount: len(s.Answers),
	}
	return nil
}

func (st *SurveyLogStore) saveFile(f io.Writer, answersCount int, s *models.Survey) error {
	for i := answersCount; i < len(s.Answers); i++ {
		bytes, err := json.Marshal(&s.Answers[i])
		if err != nil {
			return err
		}
		bytes = append(bytes, []byte("\n")...)
		_, err = f.Write(bytes)
		if err != nil {
			return err
		}
	}
	return nil
}

func createOrOpen(path string, create, open func(string) error) error {
	fi, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		// ok the file does not exist
		return create(path)
	} else if err != nil {
		return err
	} else if fi.IsDir() {
		return fmt.Errorf("%v is a directory", path)
	} else {
		return open(path)
	}
}

