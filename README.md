# Google Workspace Email Migrator

This tool is designed for migrations from local storage (i.e a POP client) to a Google Workspace account.

It requires a service account credential (generated in the Google Cloud Console) that has domain-wide delegation on the target user's domain and the following scopes:

- `https://www.googleapis.com/auth/gmail.insert`
- `https://www.googleapis.com/auth/gmail.readonly`
- `https://www.googleapis.com/auth/apps.groups.migration`

The creation of this tool was inspired by the fact (or my lack of searching abilities) that no free tool for such goal exists (besides batching GAM?).


## Inner Working

After running the command `gmm.exe setup [-v]` the application creates a SQLite database (`./manager.db`), a `./emls` folder and expects a file `./credentials.json` containing the default structure for a service account credentials file.

This database tracks the state of the migration state for each file.

Because one email can have multiple recipients, one file can be mapped to distinct combinations of `target`/`destination`, where `target` is intended to be the user account on the Google Workspace Directory and `destination` the email's recipient that the entry is related to. This `destination` field is more of a control field and does not require the address provided to be among the email's actual recipients as of the latest version. 

## Usage

Help can be found by running `gmm.exe help` while I don't write a more extensive documentation.

### Quick Guide

1. Create a Project in Google Cloud Platform
2. Enable the GMail API and Groups Migration API
3. Go to 'IAM & Admin -> Service Accounts' and create a Service Account.

> [!WARN]
> Do not provide roles to the service account. The access will be provided by domain-wide delegation.

4. Take note of the OAuth Client ID of the account just created.
5. Go to the Workspace admin console at admin.google.com, navigate to 'Security -> Access and data control -> API controls'.
6. Click in 'Manage Domain Wide Delegation' (enable it if not enabled yet) then 'Add New'.
7. Paste the 'Client ID' noted in the account creation in the first field, and the scopes noted at the beginning of this file comma separated in the second field.

> [!INFO] 
> **If your organization was created after May 3, 2024**
> You might need to disable the enforcement of the `iam.disableServiceAccountKeyCreation` and `iam.managed.disableServiceAccountKeyCreation`
> When doing so, select the project you created for this purpose and **disable the enforcement only for this project**.

8. Go to 'IAM & Admin -> Service Accounts', select the service account you created and go to the 'Keys' tab and create a new JSON key. The key will be auto downloaded. Get that file and put it in the same folder as the `gmm.exe`. Rename it to `credentials.json`.
9. Run the `gmm.exe setup` command.
10. Put the emails you want to migrate in folders (each target user account must have its own folder) within `./emls` and map these folders to a target address using `gmm.exe map folder` command.
11. Finally, migrate them using either `gmm.exe migrate user` or `gmm.exe migrate group`.

### Commands Specifications
