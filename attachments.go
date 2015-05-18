package libmail

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"sync"
)

var (
	once sync.Once
)

type AttachmentFileKind string
type AttachmentContentDisposition string

const (
	Bytes      = AttachmentFileKind("bytes")
	Path       = AttachmentFileKind("path")
	Inline     = AttachmentContentDisposition("inline")
	Attachment = AttachmentContentDisposition("attachment")
)

type AttachmentInfo struct {
	StreamKind         AttachmentFileKind
	Bytes              []byte
	Name               string
	MimeType           string
	Path               string // path in disk
	ContentDisposition AttachmentContentDisposition
}

func initMIME() {
	//TODO: add more mime types than go has
}

func guessMIME(ai *AttachmentInfo) {
	once.Do(initMIME)
	if len(ai.MimeType) > 0 {
		return
	}
	if ai.StreamKind == Path {
		ext := path.Ext(ai.Path)
		mt := mime.TypeByExtension(ext)
		ai.MimeType = mt
		if len(mt) > 0 {
			return
		}
	}
	stream, err := ai.GetStream()
	if err != nil {
		return
	}
	defer stream.Close()
	bs := make([]byte, 512)
	stream.Read(bs)
	ai.MimeType = http.DetectContentType(bs)
}

func NewAttachmentBytes(bs []byte, name string, mime string) (f *AttachmentInfo) {
	f = &AttachmentInfo{}
	f.ContentDisposition = Attachment
	f.Name = name
	f.Bytes = bs
	f.StreamKind = Bytes
	f.MimeType = mime
	guessMIME(f)
	return f
}

func NewAttachmentPath(path string, name string, mime string) (f *AttachmentInfo) {
	f = &AttachmentInfo{}
	f.ContentDisposition = Attachment
	f.Name = name
	f.Path = path
	f.StreamKind = Path
	f.MimeType = mime
	guessMIME(f)
	return f
}

//TODO: add inline attachments

func (a *AttachmentInfo) GetStream() (io.ReadCloser, error) {
	switch a.StreamKind {
	case Bytes:
		if a.Bytes == nil {
			return nil, errors.New("bytes are nil")
		}
		return NewReadCloserBuffer(a.Bytes), nil
	case Path:
		f, err := os.Open(a.Path)
		return f, err
	}
	return nil, errors.New("invalid stream kind")
}

func NewAttachmentList() *AttachmentList {
	v := &AttachmentList{}
	v.count = 0
	return v
}

type AttachmentList struct {
	count int
	first *AttachmentListItem
	last  *AttachmentListItem
}

func (l *AttachmentList) Add(item *AttachmentInfo) {
	ni := &AttachmentListItem{}
	ni.Value = item
	if l.first == nil {
		l.first = ni
	}
	if l.last != nil {
		l.last.next = ni
		ni.prev = l.last
	}
	l.last = ni
	l.count++
}

func (l *AttachmentList) First() *AttachmentListItem {
	return l.first
}

func (l *AttachmentList) Last() *AttachmentListItem {
	return l.last
}

func (l *AttachmentList) Count() int {
	return l.count
}

func (l *AttachmentList) GetFilenames() []string {
	//P141006 fixed memory leak
	names := make([]string, 0)
	for li := l.First(); li != nil; li = li.Next() {
		names = append(names, li.Value.Name)
	}
	return names
}

type AttachmentListItem struct {
	Value *AttachmentInfo
	next  *AttachmentListItem
	prev  *AttachmentListItem
}

func (q *AttachmentListItem) Next() *AttachmentListItem {
	return q.next
}
