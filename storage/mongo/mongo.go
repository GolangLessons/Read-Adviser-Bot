package mongo

import (
	"context"
	"errors"
	"io"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"read-adviser-bot/lib/e"
	"read-adviser-bot/storage"
)

type Storage struct {
	pages Pages
}

type Pages struct {
	*mongo.Collection
}

type Page struct {
	URL      string `bson:"url"`
	UserName string `bson:"username"`
}

func New(connectString string, connectTimeout time.Duration) Storage {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(connectString))
	if err != nil {
		log.Fatal(err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal(err)
	}

	pages := Pages{
		Collection: client.Database("read-adviser").Collection("pages"),
	}

	return Storage{
		pages: pages,
	}
}

func (s Storage) Save(ctx context.Context, page *storage.Page) error {
	_, err := s.pages.InsertOne(ctx, Page{
		URL:      page.URL,
		UserName: page.UserName,
	})
	if err != nil {
		return e.Wrap("can't save page", err)
	}

	return nil
}

func (s Storage) PickRandom(ctx context.Context, userName string) (page *storage.Page, err error) {
	defer func() { err = e.WrapIfErr("can't pick random page", err) }()

	pipe := bson.A{
		bson.M{"$sample": bson.M{"size": 1}},
	}

	cursor, err := s.pages.Aggregate(ctx, pipe)
	if err != nil {
		return nil, err
	}

	var p Page

	cursor.Next(ctx)

	err = cursor.Decode(&p)
	switch {
	case errors.Is(err, io.EOF):
		return nil, storage.ErrNoSavedPages
	case err != nil:
		return nil, err
	}

	return &storage.Page{
		URL:      p.URL,
		UserName: p.UserName,
	}, nil
}

func (s Storage) Remove(ctx context.Context, storagePage *storage.Page) error {
	_, err := s.pages.DeleteOne(ctx, toPage(storagePage).Filter())
	if err != nil {
		return e.Wrap("can't remove page", err)
	}

	return nil
}

func (s Storage) IsExists(ctx context.Context, storagePage *storage.Page) (bool, error) {
	count, err := s.pages.CountDocuments(ctx, toPage(storagePage).Filter())
	if err != nil {
		return false, e.Wrap("can't check if page exists", err)
	}

	return count > 0, nil
}

func toPage(p *storage.Page) Page {
	return Page{
		URL:      p.URL,
		UserName: p.UserName,
	}
}

func (p Page) Filter() bson.M {
	return bson.M{
		"url":      p.URL,
		"username": p.UserName,
	}
}
