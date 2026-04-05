package hasura

import (
	"context"
	"errors"
	"net/http"
	"time"

	"hiring-challenge-backend/api/internal/hasuragql"

	"github.com/google/uuid"
	"github.com/maaft/gqlgenc/client"
	"github.com/maaft/gqlgenc/client/transport"
)

var ErrNotFound = errors.New("record not found")

type Client struct {
	gql *hasuragql.Client
}

func NewClient(url, adminSecret string) *Client {
	httpTransport := &transport.Http{
		URL: url,
		RequestOptions: []transport.HttpRequestOption{
			func(req *http.Request) {
				req.Header.Set("X-Hasura-Admin-Secret", adminSecret)
			},
		},
	}

	return &Client{
		gql: &hasuragql.Client{
			Client: &client.Client{
				Transport: httpTransport,
			},
		},
	}
}

type User struct {
	ID             uuid.UUID
	Email          string
	DisplayName    string
	PasswordHash   string
	DefaultGroupID uuid.UUID
}

type InsertImageNodeInput struct {
	ProjectID    uuid.UUID
	ParentID     *uuid.UUID
	GroupID      uuid.UUID
	CreatedBy    uuid.UUID
	Name         string
	MimeType     string
	SizeBytes    int64
	StorageKey   string
	ThumbnailKey string
	CreatedAt    time.Time
}

type Project struct {
	ID      uuid.UUID
	GroupID uuid.UUID
}

func (c *Client) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	response, _, err := c.gql.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if len(response.Users) == 0 {
		return nil, ErrNotFound
	}

	user := response.Users[0]
	return &User{
		ID:             user.ID,
		Email:          user.Email,
		DisplayName:    user.DisplayName,
		PasswordHash:   user.PasswordHash,
		DefaultGroupID: user.DefaultGroupID,
	}, nil
}

func (c *Client) UserHasGroupMembership(ctx context.Context, userID, groupID uuid.UUID) (bool, error) {
	response, _, err := c.gql.CheckUserGroupMembership(ctx, userID, groupID)
	if err != nil {
		return false, err
	}
	return len(response.GroupMemberships) > 0, nil
}

func (c *Client) GetProjectForUser(ctx context.Context, projectID, userID uuid.UUID) (*Project, error) {
	response, _, err := c.gql.GetProjectForUser(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	if len(response.Projects) == 0 {
		return nil, ErrNotFound
	}

	project := response.Projects[0]
	return &Project{
		ID:      project.ID,
		GroupID: project.GroupID,
	}, nil
}

func (c *Client) UpdateUserLastLogin(ctx context.Context, userID uuid.UUID, at time.Time) error {
	_, _, err := c.gql.UpdateUserLastLogin(ctx, userID, at)
	return err
}

func (c *Client) InsertImageNode(ctx context.Context, input InsertImageNodeInput) (uuid.UUID, error) {
	nodeID := uuid.New()
	response, _, err := c.gql.InsertImageNodeAndData(
		ctx,
		nodeID,
		input.ProjectID,
		input.ParentID,
		input.GroupID,
		input.CreatedBy,
		input.Name,
		input.CreatedAt,
		input.MimeType,
		input.SizeBytes,
		input.StorageKey,
		input.ThumbnailKey,
	)
	if err != nil {
		return uuid.Nil, err
	}
	if response.InsertNodesOne == nil {
		return uuid.Nil, errors.New("insert_nodes_one returned null")
	}
	return response.InsertNodesOne.ID, nil
}
