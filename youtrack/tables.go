package youtrack

import (
	"github.com/turbot/steampipe-plugin-sdk/v6/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/v6/plugin"
	"github.com/turbot/steampipe-plugin-sdk/v6/plugin/transform"
)

func resourceDefinitions() []resourceDefinition {
	return []resourceDefinition{
		{name: "youtrack_project", description: "Projects visible to the connection user.", path: []string{"admin", "projects"}, fields: []string{"id,name,shortName,description,leader(id,login,fullName)"}, columns: projectColumns(), getKeys: []string{"id", "short_name"}},
		{name: "youtrack_issue", description: "Issues visible to the connection user.", path: []string{"issues"}, fields: []string{"id,idReadable,summary,description,project(id,name,shortName),reporter(id,login,fullName),updater(id,login,fullName),created,updated,resolved,isDraft,tags(id,name),customFields(id,name,$type,value(id,name,login,fullName,text,minutes,presentation)),commentsCount,votes"}, columns: issueColumns(), listKeys: plugin.OptionalColumns([]string{"query", "project_id"}), getKeys: []string{"id", "id_readable"}},
		{name: "youtrack_user", description: "Users visible through the current YouTrack API.", path: []string{"users"}, fields: []string{"id,login,name,fullName,email,banned,online"}, columns: userColumns(), getKeys: []string{"id", "login"}},
		{name: "youtrack_group", description: "User groups visible through the current YouTrack API.", path: []string{"groups"}, fields: []string{"id,name,description,users(id,login,fullName)"}, columns: groupColumns(), listKeys: plugin.OptionalColumns([]string{"query"})},
		{name: "youtrack_tag", description: "Tags visible to the connection user.", path: []string{"tags"}, fields: []string{"id,name,owner(id,login,fullName),untagOnResolve,readSharingSettings,tagSharingSettings,updateSharingSettings"}, columns: tagColumns(), listKeys: plugin.OptionalColumns([]string{"query"})},
		{name: "youtrack_saved_query", description: "Saved issue searches visible to the connection user.", path: []string{"savedQueries"}, fields: []string{"id,name,query,owner(id,login,fullName),readSharingSettings,updateSharingSettings"}, columns: savedQueryColumns()},
		{name: "youtrack_article", description: "Knowledge base articles visible to the connection user.", path: []string{"articles"}, fields: []string{"id,idReadable,summary,content,project(id,name,shortName),reporter(id,login,fullName),created,updated,tags(id,name)"}, columns: articleColumns(), getKeys: []string{"id", "id_readable"}},
		{name: "youtrack_agile", description: "Agile boards visible to the connection user.", path: []string{"agiles"}, fields: []string{"id,name,owner(id,login,fullName),projects(id,name,shortName),sprints(id,name),currentSprint(id,name)"}, columns: agileColumns()},
		{name: "youtrack_issue_comment", description: "Issue comments visible to the connection user.", path: []string{"issues"}, parentKey: "issue_id", parentPath: []string{"comments"}, fields: []string{"id,text,author(id,login,fullName),created,updated,issue(id,idReadable,summary)"}, columns: commentColumns(), listKeys: plugin.SingleColumn("issue_id")},
		{name: "youtrack_issue_work_item", description: "Issue work items visible to the connection user.", path: []string{"workItems"}, fields: []string{"id,text,author(id,login,fullName),creator(id,login,fullName),date,created,updated,duration(minutes,presentation),issue(id,idReadable,summary)"}, columns: workItemColumns(), listKeys: plugin.OptionalColumns([]string{"issue_id", "query", "start_date", "end_date", "start", "end", "created_start", "created_end", "updated_start", "updated_end", "author_filter", "creator_filter"})},
	}
}

func projectColumns() []*plugin.Column {
	return []*plugin.Column{
		{Name: "name", Type: proto.ColumnType_STRING, Description: "The project name."},
		{Name: "short_name", Type: proto.ColumnType_STRING, Description: "The project's short name."},
		{Name: "description", Type: proto.ColumnType_STRING, Description: "The project description."},
		{Name: "leader", Type: proto.ColumnType_JSON, Description: "The project leader."},
	}
}

func issueColumns() []*plugin.Column {
	return []*plugin.Column{
		{Name: "id_readable", Type: proto.ColumnType_STRING, Description: "The human-readable issue ID.", Transform: transform.FromField("IDReadable")},
		{Name: "summary", Type: proto.ColumnType_STRING, Description: "The issue summary."},
		{Name: "description", Type: proto.ColumnType_STRING, Description: "The issue description; null when unset."},
		{Name: "project", Type: proto.ColumnType_JSON, Description: "The issue project."},
		{Name: "project_id", Type: proto.ColumnType_STRING, Description: "The issue project's database ID; an exact qualifier uses the project-scoped endpoint.", Transform: transform.FromField("ProjectID")},
		{Name: "reporter", Type: proto.ColumnType_JSON, Description: "The issue reporter."},
		{Name: "updater", Type: proto.ColumnType_JSON, Description: "The last updater."},
		{Name: "created", Type: proto.ColumnType_TIMESTAMP, Description: "When the issue was created.", Transform: transform.FromField("Created").Transform(milliseconds)},
		{Name: "updated", Type: proto.ColumnType_TIMESTAMP, Description: "When the issue was last updated.", Transform: transform.FromField("Updated").Transform(milliseconds)},
		{Name: "resolved", Type: proto.ColumnType_TIMESTAMP, Description: "When the issue was resolved; null when unresolved.", Transform: transform.FromField("Resolved").Transform(milliseconds)},
		{Name: "is_draft", Type: proto.ColumnType_BOOL, Description: "Whether the issue is a draft."},
		{Name: "tags", Type: proto.ColumnType_JSON, Description: "Tags attached to the issue."},
		{Name: "custom_fields", Type: proto.ColumnType_JSON, Description: "Raw polymorphic custom-field values."},
		{Name: "comments_count", Type: proto.ColumnType_INT, Description: "The number of comments."},
		{Name: "votes", Type: proto.ColumnType_INT, Description: "The number of votes."},
		{Name: "query", Type: proto.ColumnType_STRING, Description: "An exact YouTrack issue query pushed to the API.", Transform: transform.FromQual("query")},
	}
}

func userColumns() []*plugin.Column {
	return []*plugin.Column{
		{Name: "name", Type: proto.ColumnType_STRING, Description: "The user name."},
		{Name: "login", Type: proto.ColumnType_STRING, Description: "The user login."},
		{Name: "full_name", Type: proto.ColumnType_STRING, Description: "The user's full name."},
		{Name: "email", Type: proto.ColumnType_STRING, Description: "The user's email when visible; null when hidden or unset."},
		{Name: "banned", Type: proto.ColumnType_BOOL, Description: "Whether the user account is banned; null when not visible."},
		{Name: "online", Type: proto.ColumnType_BOOL, Description: "Whether the user is online; null when not visible."},
	}
}
func groupColumns() []*plugin.Column {
	return []*plugin.Column{
		{Name: "name", Type: proto.ColumnType_STRING, Description: "The group name."},
		{Name: "description", Type: proto.ColumnType_STRING, Description: "The group description."},
		{Name: "users", Type: proto.ColumnType_JSON, Description: "Users visible in the group."},
		{Name: "query", Type: proto.ColumnType_STRING, Description: "An exact group query pushed to the API.", Transform: transform.FromQual("query")},
	}
}
func tagColumns() []*plugin.Column {
	return []*plugin.Column{
		{Name: "name", Type: proto.ColumnType_STRING, Description: "The tag name."},
		{Name: "owner", Type: proto.ColumnType_JSON, Description: "The tag owner."},
		{Name: "untag_on_resolve", Type: proto.ColumnType_BOOL, Description: "Whether the tag is removed when an issue is resolved."},
		{Name: "read_sharing_settings", Type: proto.ColumnType_JSON, Description: "The tag visibility settings."},
		{Name: "tag_sharing_settings", Type: proto.ColumnType_JSON, Description: "The settings controlling who can use the tag."},
		{Name: "update_sharing_settings", Type: proto.ColumnType_JSON, Description: "The settings controlling who can update the tag."},
		{Name: "query", Type: proto.ColumnType_STRING, Description: "An exact tag query pushed to the API.", Transform: transform.FromQual("query")},
	}
}
func savedQueryColumns() []*plugin.Column {
	return []*plugin.Column{
		{Name: "name", Type: proto.ColumnType_STRING, Description: "The saved query name."},
		{Name: "query_text", Type: proto.ColumnType_STRING, Description: "The stored YouTrack issue query.", Transform: transform.FromField("Query")},
		{Name: "owner", Type: proto.ColumnType_JSON, Description: "The saved query owner."},
		{Name: "read_sharing_settings", Type: proto.ColumnType_JSON, Description: "The saved query visibility settings."},
		{Name: "update_sharing_settings", Type: proto.ColumnType_JSON, Description: "The settings controlling who can update the saved query."},
	}
}
func articleColumns() []*plugin.Column {
	return []*plugin.Column{
		{Name: "id_readable", Type: proto.ColumnType_STRING, Description: "The human-readable article ID.", Transform: transform.FromField("IDReadable")},
		{Name: "summary", Type: proto.ColumnType_STRING, Description: "The article summary."},
		{Name: "content", Type: proto.ColumnType_STRING, Description: "The article content."},
		{Name: "project", Type: proto.ColumnType_JSON, Description: "The article project."},
		{Name: "reporter", Type: proto.ColumnType_JSON, Description: "The article reporter."},
		{Name: "created", Type: proto.ColumnType_TIMESTAMP, Description: "When the article was created.", Transform: transform.FromField("Created").Transform(milliseconds)},
		{Name: "updated", Type: proto.ColumnType_TIMESTAMP, Description: "When the article was last updated.", Transform: transform.FromField("Updated").Transform(milliseconds)},
		{Name: "tags", Type: proto.ColumnType_JSON, Description: "Article tags."},
	}
}
func agileColumns() []*plugin.Column {
	return []*plugin.Column{
		{Name: "name", Type: proto.ColumnType_STRING, Description: "The agile board name."},
		{Name: "owner", Type: proto.ColumnType_JSON, Description: "The agile board owner."},
		{Name: "projects", Type: proto.ColumnType_JSON, Description: "Projects associated with the board."},
		{Name: "sprints", Type: proto.ColumnType_JSON, Description: "Sprints visible on the board."},
		{Name: "current_sprint", Type: proto.ColumnType_JSON, Description: "The board's current sprint."},
	}
}
func commentColumns() []*plugin.Column {
	return []*plugin.Column{{Name: "issue_id", Type: proto.ColumnType_STRING, Description: "The required parent issue ID.", Transform: transform.FromQual("issue_id")}, {Name: "text", Type: proto.ColumnType_STRING, Description: "The comment text."}, {Name: "author", Type: proto.ColumnType_JSON, Description: "The comment author."}, {Name: "issue", Type: proto.ColumnType_JSON, Description: "The parent issue."}, {Name: "created", Type: proto.ColumnType_TIMESTAMP, Description: "When the comment was created.", Transform: transform.FromField("Created").Transform(milliseconds)}, {Name: "updated", Type: proto.ColumnType_TIMESTAMP, Description: "When the comment was last updated.", Transform: transform.FromField("Updated").Transform(milliseconds)}}
}
func workItemColumns() []*plugin.Column {
	return []*plugin.Column{
		{Name: "issue_id", Type: proto.ColumnType_STRING, Description: "The parent issue ID.", Transform: transform.FromField("IssueID")},
		{Name: "text", Type: proto.ColumnType_STRING, Description: "The work item text."},
		{Name: "author", Type: proto.ColumnType_JSON, Description: "The work item author."},
		{Name: "creator", Type: proto.ColumnType_JSON, Description: "The user who created the work item."},
		{Name: "issue", Type: proto.ColumnType_JSON, Description: "The parent issue."},
		{Name: "duration", Type: proto.ColumnType_JSON, Description: "The recorded duration."},
		{Name: "date", Type: proto.ColumnType_TIMESTAMP, Description: "The work date.", Transform: transform.FromField("Date").Transform(milliseconds)},
		{Name: "created", Type: proto.ColumnType_TIMESTAMP, Description: "When the work item was created.", Transform: transform.FromField("Created").Transform(milliseconds)},
		{Name: "updated", Type: proto.ColumnType_TIMESTAMP, Description: "When the work item was last updated.", Transform: transform.FromField("Updated").Transform(milliseconds)},
		{Name: "query", Type: proto.ColumnType_STRING, Description: "An exact issue search query pushed to the global work-item API.", Transform: transform.FromQual("query")},
		{Name: "start_date", Type: proto.ColumnType_TIMESTAMP, Description: "Inclusive work date lower bound, sent as YYYY-MM-DD.", Transform: transform.FromQual("start_date")},
		{Name: "end_date", Type: proto.ColumnType_TIMESTAMP, Description: "Inclusive work date upper bound, sent as YYYY-MM-DD.", Transform: transform.FromQual("end_date")},
		{Name: "start", Type: proto.ColumnType_TIMESTAMP, Description: "Work timestamp lower bound, sent as Unix milliseconds.", Transform: transform.FromQual("start")},
		{Name: "end", Type: proto.ColumnType_TIMESTAMP, Description: "Work timestamp upper bound, sent as Unix milliseconds.", Transform: transform.FromQual("end")},
		{Name: "created_start", Type: proto.ColumnType_TIMESTAMP, Description: "Creation timestamp lower bound, sent as Unix milliseconds.", Transform: transform.FromQual("created_start")},
		{Name: "created_end", Type: proto.ColumnType_TIMESTAMP, Description: "Creation timestamp upper bound, sent as Unix milliseconds.", Transform: transform.FromQual("created_end")},
		{Name: "updated_start", Type: proto.ColumnType_TIMESTAMP, Description: "Update timestamp lower bound, sent as Unix milliseconds.", Transform: transform.FromQual("updated_start")},
		{Name: "updated_end", Type: proto.ColumnType_TIMESTAMP, Description: "Update timestamp upper bound, sent as Unix milliseconds.", Transform: transform.FromQual("updated_end")},
		{Name: "author_filter", Type: proto.ColumnType_STRING, Description: "One or more author IDs or logins pushed to the global work-item API.", Transform: transform.FromQual("author_filter")},
		{Name: "creator_filter", Type: proto.ColumnType_STRING, Description: "One or more creator IDs or logins pushed to the global work-item API.", Transform: transform.FromQual("creator_filter")},
	}
}
