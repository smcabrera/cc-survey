package models

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
)

import (
	"github.com/gorilla/schema"
)

import (
	"github.com/timtadh/cc-survey/clones"
)


type Question struct {
	Name string
	Question string
	Required bool
}

type MultipleChoice struct {
	Question
	Answers []Answer
}

type Answer struct {
	Value string
	Answer string
}

type FreeResponse struct {
	Question
	MaxLength int
}

type Form struct {
	Action string
	Csrf string
	SubmitText string
	Questions []Renderable
}

type Renderable interface {
	Key() string
	HTML(err error) template.HTML
}

var freeTmpl = template.Must(template.New("freeResponse").Parse(
`<label>
	<div class="question{{if .question.Required}} required{{end}}">
		{{.question.Question.Question}} {{if .error}} <div class="error">{{.error}}</div> {{end}}
	</div>
	<div class="answer">
		<textarea name="{{.question.Name}}" maxlength={{.question.MaxLength}} cols="60" rows="6"></textarea>
	</div>
</label>`))

var multiTmpl = template.Must(template.New("multipleChoice").Parse(
`<div class="question{{if .question.Required}} required{{end}}">
		{{.question.Question.Question}} {{if .error}} <div class="error">{{.error}}</div> {{end}}
	</div>{{$q := .question}}{{range $a := $q.Answers}}
	<div class="answer">
	<label>
		<input type="radio" name="{{$q.Name}}" value="{{$a.Value}}"/>
		{{$a.Answer}}
	</label>
</div>{{end}}`))

var formTmpl = template.Must(template.New("form").Parse(
`<form class="survey" action="{{.form.Action}}" method="post">{{$e := .errors}}{{range $q := .form.Questions}}
{{$q.HTML (index $e $q.Key)}}{{end}}
<input type="hidden" name="csrf" value="{{.form.Csrf}}"/>
<div class="submit"><input type="submit" value="{{.form.SubmitText}}"/></div>
</form>`))


func (f *Form) Decode(s *Session, u *User, c *clones.Clone, cid int, r *http.Request) (*SurveyAnswer, schema.MultiError, error) {
	err := r.ParseForm()
	if err != nil {
		return nil, nil, err
	}
	answer := &SurveyAnswer{
		UserEmail: u.Email,
		CloneID: cid,
		CloneExtID: c.ExtId(),
		CloneDir: c.Dir(),
		SelectionPr: c.Pr(),
		Responses: make([]Response, 0, len(f.Questions)),
	}
	errors := make(schema.MultiError)
	form := r.PostForm
	if value, has := form["csrf"]; !has {
		errors["csrf"] = fmt.Errorf("invalid csrf token")
	} else if !s.ValidCsrf(r.URL.Path, strings.Join(value, "")) {
		errors["csrf"] = fmt.Errorf("invalid csrf token")
	}
	for qid, r := range f.Questions {
		switch q := r.(type) {
		case *MultipleChoice:
			if value, has := form[q.Name]; !has && q.Required {
				errors[q.Name] = fmt.Errorf("This is a required question")
				answer.Responses = append(answer.Responses, Response{
					QuestionID: qid,
					Answer: -1,
					Text: "Not Answered",
				})
			} else if !has {
				answer.Responses = append(answer.Responses, Response{
					QuestionID: qid,
					Answer: -1,
					Text: "Not Answered",
				})
			} else {
				aid, err := q.AnswerNumber(value[0])
				if err != nil {
					errors[q.Name] = err
					answer.Responses = append(answer.Responses, Response{
						QuestionID: qid,
						Answer: -2,
						Text: "Bad Answer",
					})
				} else {
					answer.Responses = append(answer.Responses, Response{
						QuestionID: qid,
						Answer: aid,
						Text: strings.Join(value, ""),
					})
				}
			}
		case *FreeResponse:
			value, has := form[q.Name]
			has = has && value[0] != ""
			if !has && q.Required {
				errors[q.Name] = fmt.Errorf("This is a required question")
				answer.Responses = append(answer.Responses, Response{
					QuestionID: qid,
					Answer: -1,
					Text: "Not Answered",
				})
			} else if !has {
				answer.Responses = append(answer.Responses, Response{
					QuestionID: qid,
					Answer: -1,
					Text: "Not Answered",
				})
			} else {
				text := strings.Join(value, "")
				if len(text) > q.MaxLength {
					errors[q.Name] = fmt.Errorf("Response was too long")
					answer.Responses = append(answer.Responses, Response{
						QuestionID: qid,
						Answer: -2,
						Text: "Bad Answer",
					})
				} else {
					answer.Responses = append(answer.Responses, Response{
						QuestionID: qid,
						Answer: -3,
						Text: text,
					})
				}
			}
		default:
			log.Panic(fmt.Errorf("unexpected question type"))
		}
	}
	return answer, errors, nil
}

func (f *Form) HTML(errs schema.MultiError) template.HTML {
	return HTML(formTmpl, map[string]interface{}{
		"form": f,
		"errors": map[string]error(errs),
	})
}

func (q *FreeResponse) Key() string {
	return q.Name
}

func (q *FreeResponse) HTML(err error) template.HTML {
	return HTML(freeTmpl, map[string]interface{}{
		"question": q,
		"error": err,
	})
}

func (q *MultipleChoice) Key() string {
	return q.Name
}

func (q *MultipleChoice) HTML(err error) template.HTML {
	return HTML(multiTmpl, map[string]interface{}{
		"question": q,
		"error": err,
	})
}

func (q *MultipleChoice) AnswerNumber(key string) (int, error) {
	for aid, a := range q.Answers {
		if key == a.Value {
			return aid, nil
		}
	}
	return -1, fmt.Errorf("Not a valid answer")
}

func HTML(t *template.Template, data interface{}) template.HTML {
	buf := new(bytes.Buffer)
	err := t.Execute(buf, data)
	if err != nil {
		log.Panic(err)
	}
	return template.HTML(buf.String())
}

