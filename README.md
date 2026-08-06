# Google Workspace Email Migrator

This tool is designed for migrations from local storage (i.e a POP client) to a Google Workspace account.

It requires a service account credential (generated in the Google Cloud Console) that has domain-wide delegation on the target user's domain and the following scopes:

- `https://www.googleapis.com/auth/gmail.insert`
- `https://www.googleapis.com/auth/gmail.readonly`

The creation of this tool was inspired by the fact (or my lack of searching abilities) that no free tool for such goal exists (besides batching GAM?).


## Inner Working

After running the command `gmm.exe setup [-v]` the application creates a SQLite database (`./manager.db`), a `./emls` folder and expects a file `./credentials.json` containing the default structure for a service account credentials file.

This database tracks the state of the migration state for each file.

Because one email can have multiple recipients, one file can be mapped to distinct combinations of `target`/`destination`, where `target` is intended to be the user account on the Google Workspace Directory and `destination` the email's recipient that the entry is related to. This `destination` field is more of a control field and does not require the address provided to be among the email's actual recipients as of the latest version. 

## Usage

Help can be found by running `gmm.exe help` while I don't write a more extensive documentation.