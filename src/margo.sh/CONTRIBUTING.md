### Introduction

Thank you for considering contributing to margo!

Although margo is officially a Kuroku Labs product, it's still an open source project, and we welcome all types of contributions - be it bug reports, marketing, code, documentation, etc.


### Contributor License Agreement (CLA)

As is the case with many Open Source projects, we can only accept source code contributions from contributors that have signed our CLA. Visit https://cla.kuroku.io/ for more details.

### Dev environment setup

The easiest way to get started with margo development is through GoSublime:

* [install GoSublime](https://github.com/DisposaBoy/GoSublime#installation) with git
* switch to the `development` branch `git checkout development`
* while in Sublime Text/GoSublime, press <kbd>ctrl+.</kbd>,<kbd>ctrl+9</kbd> (<kbd>cmd+.</kbd>,<kbd>cmd+9</kbd> on Mac) to open the GoSublime command prompt and run the command `margo.sh dev fork $your-fork` e.g. `margo.sh dev fork git@github.com:DisposaBoy/margo.git`

  this sets the git remote `margo` to the upstream repo from which you will `pull` your updates.
  the `origin` remote is set to your fork to which you will push your changes.

### Your First Contribution

Working on your first Pull Request? You can learn how from this *free* series, [How to Contribute to an Open Source Project on GitHub](https://egghead.io/series/how-to-contribute-to-an-open-source-project-on-github).

### Submitting code

Any code change should be submitted as a pull request. The description should explain what the code does and give steps to execute it. The pull request should ideally also contain tests.

### Code review process

The bigger the pull request, the longer it will take to review and merge. Try to break down large pull requests in smaller chunks that are easier to review and merge.
It is also always helpful to have some context for your pull request. What was the purpose? Why does it matter to you?




<!-- This `CONTRIBUTING.md` is based on @nayafia's template https://github.com/nayafia/contributing-template -->
