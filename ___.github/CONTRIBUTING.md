##### SPDX-FileCopyrightText: (C) 2025 Intel Corporation
##### SPDX-License-Identifier: Apache-2.0
------------------------------------------------------------------

## <a name="commit"></a> Commit Message Guidelines

We have precise rules over how our git commit messages should be formatted.  This leads to more readable messages that are easy to follow when looking through the project history.

### Commit Message Format
Each commit message consists of a **header**, a **body** and a **footer**.  The header has a special format that includes a **JIRA ticket** and a **subject**:

```
[SL6-****] <subject>
<BLANK LINE>
<body>
<BLANK LINE>
<footer>
```

Any line of the commit message should not be longer than 50 characters max on the subject line and 72 characters max on the body! This allows the message to be easier to read on GitHub as well as in various git tools.

Example:
```
[SL6-1000]: Introduce new ROS parameter for client node

In order to give a user option to set value X, a new ROS
parameter has been introduced as Xvalue.
Corresponding tests and docs updated

```

### Pull Requests practices

* PR author is responsible to merge its own PR after review has been done and CI has passed.
* When merging, make sure git linear history is preserved. PR author should select a merge option (`Rebase and merge` or `Squash and merge`) based on which option will fit the best to the git linear history.
