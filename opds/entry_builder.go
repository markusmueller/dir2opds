package opds

import (
	"time"

	"github.com/lann/builder"
	"golang.org/x/tools/blog/atom"
)

type EntryBuilder builder.Builder

func (e EntryBuilder) Title(title string) EntryBuilder {
	return builder.Set(e, "Title", title).(EntryBuilder)
}

func (e EntryBuilder) ID(id string) EntryBuilder {
	return builder.Set(e, "ID", id).(EntryBuilder)
}

func (e EntryBuilder) AddLink(link atom.Link) EntryBuilder {
	return builder.Append(e, "Link", link).(EntryBuilder)
}

func (e EntryBuilder) Published(published time.Time) EntryBuilder {
	return builder.Set(e, "Published", atom.Time(published)).(EntryBuilder)
}

func (e EntryBuilder) Updated(updated time.Time) EntryBuilder {
	return builder.Set(e, "Updated", atom.Time(updated)).(EntryBuilder)
}

func (e EntryBuilder) Author(author *atom.Person) EntryBuilder {
	return builder.Set(e, "Author", author).(EntryBuilder)
}

func (e EntryBuilder) Summary(summary *atom.Text) EntryBuilder {
	return builder.Set(e, "Summary", summary).(EntryBuilder)
}

func (e EntryBuilder) Content(content *atom.Text) EntryBuilder {
	return builder.Set(e, "Content", content).(EntryBuilder)
}

func (e EntryBuilder) Build() atom.Entry {
	return builder.GetStruct(e).(atom.Entry)
}

// Builder is a fluent immutable builder to build OPDS entries
var Builder = builder.Register(EntryBuilder{}, atom.Entry{}).(EntryBuilder)
