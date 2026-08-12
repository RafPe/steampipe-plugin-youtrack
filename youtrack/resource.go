package youtrack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/RafPe/steampipe-plugin-youtrack/youtrack/client"
	"github.com/turbot/steampipe-plugin-sdk/v6/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/v6/plugin"
	"github.com/turbot/steampipe-plugin-sdk/v6/plugin/transform"
)

type resource struct {
	ID                    string          `json:"id"`
	IDReadable            string          `json:"idReadable"`
	Name                  string          `json:"name"`
	ShortName             string          `json:"shortName"`
	Login                 string          `json:"login"`
	FullName              string          `json:"fullName"`
	Email                 *string         `json:"email"`
	Summary               *string         `json:"summary"`
	Description           *string         `json:"description"`
	Text                  *string         `json:"text"`
	Content               *string         `json:"content"`
	Project               json.RawMessage `json:"project"`
	ProjectID             string          `json:"-"`
	Reporter              json.RawMessage `json:"reporter"`
	Updater               json.RawMessage `json:"updater"`
	Author                json.RawMessage `json:"author"`
	Creator               json.RawMessage `json:"creator"`
	Owner                 json.RawMessage `json:"owner"`
	Leader                json.RawMessage `json:"leader"`
	Users                 json.RawMessage `json:"users"`
	Tags                  json.RawMessage `json:"tags"`
	Projects              json.RawMessage `json:"projects"`
	Sprints               json.RawMessage `json:"sprints"`
	CurrentSprint         json.RawMessage `json:"currentSprint"`
	CustomFields          json.RawMessage `json:"customFields"`
	ReadSharingSettings   json.RawMessage `json:"readSharingSettings"`
	TagSharingSettings    json.RawMessage `json:"tagSharingSettings"`
	UpdateSharingSettings json.RawMessage `json:"updateSharingSettings"`
	Created               *int64          `json:"created"`
	Updated               *int64          `json:"updated"`
	Resolved              *int64          `json:"resolved"`
	IsDraft               bool            `json:"isDraft"`
	Banned                *bool           `json:"banned"`
	Online                *bool           `json:"online"`
	UntagOnResolve        *bool           `json:"untagOnResolve"`
	CommentsCount         int64           `json:"commentsCount"`
	Votes                 int64           `json:"votes"`
	Query                 string          `json:"query"`
	Duration              json.RawMessage `json:"duration"`
	Date                  *int64          `json:"date"`
	Issue                 json.RawMessage `json:"issue"`
	IssueID               string          `json:"-"`
	Raw                   json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes a resource and retains its original JSON representation.
func (r *resource) UnmarshalJSON(data []byte) error {
	type plain resource
	if err := json.Unmarshal(data, (*plain)(r)); err != nil {
		return err
	}
	r.Raw = append(r.Raw[:0], data...)
	r.ProjectID = nestedID(r.Project)
	r.IssueID = nestedID(r.Issue)
	return nil
}

func nestedID(data json.RawMessage) string {
	var value struct {
		ID string `json:"id"`
	}
	if len(data) == 0 || json.Unmarshal(data, &value) != nil {
		return ""
	}
	return value.ID
}

type resourceDefinition struct {
	name        string
	description string
	path        []string
	parentPath  []string
	fields      []string
	columns     []*plugin.Column
	listKeys    plugin.KeyColumnSlice
	getKeys     []string
	parentKey   string
}

func tableMap(ctx context.Context) map[string]*plugin.Table {
	definitions := resourceDefinitions()
	tables := make(map[string]*plugin.Table, len(definitions))
	for _, definition := range definitions {
		definition := definition
		tables[definition.name] = resourceTable(ctx, definition)
	}
	return tables
}

func resourceTable(_ context.Context, definition resourceDefinition) *plugin.Table {
	columns := append(commonColumns(), definition.columns...)
	return &plugin.Table{
		Name:        definition.name,
		Description: definition.description,
		List: &plugin.ListConfig{
			Hydrate:    resourceList(definition),
			KeyColumns: definition.listKeys,
		},
		Get: &plugin.GetConfig{
			Hydrate:    resourceGet(definition),
			KeyColumns: getKeyColumns(definition),
		},
		Columns: columns,
	}
}

func resourceList(definition resourceDefinition) plugin.HydrateFunc {
	return func(ctx context.Context, d *plugin.QueryData, _ *plugin.HydrateData) (any, error) {
		apiClient, err := queryClient(d)
		if err != nil {
			return nil, err
		}
		query := make(url.Values)
		if pushedQuery := d.EqualsQualString("query"); pushedQuery != "" {
			query.Set("query", pushedQuery)
		}
		if definition.name == "youtrack_issue_work_item" && d.EqualsQualString("issue_id") == "" {
			addWorkItemQualifiers(query, d)
		}
		limit := int(d.QueryContext.GetLimit())
		if limit < 0 {
			limit = 0
		}
		path := resourcePath(definition, d)
		if path == nil {
			return nil, nil
		}
		var rows []resource
		if err := apiClient.List(ctx, path, query, definition.fields, limit, &rows); err != nil {
			return nil, fmt.Errorf("list %s: %w", definition.name, err)
		}
		for i := range rows {
			d.StreamListItem(ctx, rows[i])
		}
		return nil, nil
	}
}

func resourceGet(definition resourceDefinition) plugin.HydrateFunc {
	return func(ctx context.Context, d *plugin.QueryData, _ *plugin.HydrateData) (any, error) {
		id := getIdentifier(definition, d)
		if id == "" {
			return nil, nil
		}
		apiClient, err := queryClient(d)
		if err != nil {
			return nil, err
		}
		path := resourcePath(definition, d)
		if path == nil {
			return nil, nil
		}
		path = append(path, id)
		var row resource
		if err := apiClient.Get(ctx, path, nil, definition.fields, &row); err != nil {
			var httpErr *client.HTTPError
			if errors.As(err, &httpErr) && httpErr.Kind == client.ErrorNotFound {
				return nil, nil
			}
			return nil, fmt.Errorf("get %s: %w", definition.name, err)
		}
		return row, nil
	}
}

func resourcePath(definition resourceDefinition, d *plugin.QueryData) []string {
	if definition.name == "youtrack_issue" {
		if projectID := d.EqualsQualString("project_id"); projectID != "" {
			return []string{"admin", "projects", projectID, "issues"}
		}
	}
	if definition.name == "youtrack_issue_work_item" {
		if issueID := d.EqualsQualString("issue_id"); issueID != "" {
			return []string{"issues", issueID, "timeTracking", "workItems"}
		}
		return []string{"workItems"}
	}
	path := append([]string(nil), definition.path...)
	if definition.parentKey == "" {
		return path
	}
	parentID := d.EqualsQualString(definition.parentKey)
	if parentID == "" {
		return nil
	}
	return append(path, append([]string{parentID}, definition.parentPath...)...)
}

func addWorkItemQualifiers(query url.Values, d *plugin.QueryData) {
	for column, parameter := range map[string]string{
		"query": "query", "start_date": "startDate", "end_date": "endDate",
	} {
		if value := qualValue(d, column); value != nil {
			if timestamp := value.GetTimestampValue(); timestamp != nil {
				query.Set(parameter, timestamp.AsTime().UTC().Format(time.DateOnly))
			} else if text := value.GetStringValue(); text != "" {
				query.Set(parameter, text)
			}
		}
	}
	for column, parameter := range map[string]string{
		"start": "start", "end": "end", "created_start": "createdStart",
		"created_end": "createdEnd", "updated_start": "updatedStart", "updated_end": "updatedEnd",
	} {
		if value := qualValue(d, column); value != nil && value.GetTimestampValue() != nil {
			query.Set(parameter, strconv.FormatInt(value.GetTimestampValue().AsTime().UnixMilli(), 10))
		}
	}
	for column, parameter := range map[string]string{"author_filter": "author", "creator_filter": "creator"} {
		value := qualValue(d, column)
		if value == nil {
			continue
		}
		if list := value.GetListValue(); list != nil {
			for _, item := range list.Values {
				if text := item.GetStringValue(); text != "" {
					query.Add(parameter, text)
				}
			}
		} else if text := value.GetStringValue(); text != "" {
			query.Add(parameter, text)
		}
	}
}

func qualValue(d *plugin.QueryData, column string) *proto.QualValue {
	return d.EqualsQuals[column]
}

func getKeyColumns(definition resourceDefinition) plugin.KeyColumnSlice {
	if definition.parentKey != "" {
		return plugin.AllColumns(append([]string{"id"}, definition.parentKey))
	}
	if len(definition.getKeys) == 0 {
		return plugin.SingleColumn("id")
	}
	return plugin.AnyColumn(definition.getKeys)
}

func getIdentifier(definition resourceDefinition, d *plugin.QueryData) string {
	keys := definition.getKeys
	if len(keys) == 0 {
		keys = []string{"id"}
	}
	for _, key := range keys {
		if value := d.EqualsQualString(key); value != "" {
			return value
		}
	}
	return ""
}

func queryClient(d *plugin.QueryData) (*client.Client, error) {
	config, ok := d.Connection.GetConfig().(Config)
	if !ok {
		return nil, errors.New("invalid YouTrack connection configuration")
	}
	config = config.withEnvFallback()
	if err := ValidateConfig(&config); err != nil {
		return nil, fmt.Errorf("validate YouTrack connection: %w", err)
	}
	return client.New(*config.BaseURL, *config.Token)
}

func commonColumns() []*plugin.Column {
	return []*plugin.Column{
		{Name: "id", Type: proto.ColumnType_STRING, Description: "The database ID of the resource.", Transform: transform.FromField("ID")},
		{Name: "raw", Type: proto.ColumnType_JSON, Description: "The complete requested YouTrack representation.", Transform: transform.FromField("Raw")},
	}
}

func milliseconds(_ context.Context, data *transform.TransformData) (any, error) {
	if data.Value == nil {
		return nil, nil
	}
	value, ok := data.Value.(*int64)
	if ok {
		if value == nil {
			return nil, nil
		}
		return time.UnixMilli(*value).UTC(), nil
	}
	milliseconds, err := strconv.ParseInt(fmt.Sprint(data.Value), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("convert YouTrack timestamp: %w", err)
	}
	return time.UnixMilli(milliseconds).UTC(), nil
}
