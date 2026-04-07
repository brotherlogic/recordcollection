---
description: When finishing a task, run the following steps:  1. Build the code to ensure it compiles successfully:    `go build`  2. Update the README with details about the new feature   // turbo 3. Create and checkout a new branch with a descriptive name:    `
---

When finishing a task, run the following steps:

1. Build the code to ensure it compiles successfully:
   `go build`

2. Update the README with details about the new feature
 
// turbo
3. Create and checkout a new branch with a descriptive name:
   `git checkout -b <descriptive-branch-name>`

// turbo
4. Commit the changes and reference any associated issue in the issue list:
   `git commit -am "<descriptive commit message>"`

// turbo
5. Push the changes to the newly created branch:
   `git push -u origin HEAD`

6. Find the newly pushed branch in a Pull Request using the gh tool - this may require some retries

7. Trigger a review by posting a comment to the Pull Request '/gemini-review' 

8. Wait for the review to appear and make an adjustment to the existing code to address the review

9. Push this change to the same branch.