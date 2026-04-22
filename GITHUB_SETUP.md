# GitHub Setup Instructions

The local Git repository is ready. Follow these steps to push to GitHub:

## Step 1: Create Repository on GitHub

1. Go to https://github.com/new
2. Create a new repository:
   - **Repository name**: `money-transfer-usecase` (or your preferred name)
   - **Description**: "Money Transfer Usecase - Backend Developer Assessment"
   - **Public**: Yes (for evaluation)
   - **Initialize with**: No (we already have commits)
3. Click "Create repository"

## Step 2: Add Remote and Push

After creating the repository, GitHub will show you commands. Use these:

```bash
cd /home/boris/Desktop/med

# Add remote (replace YOUR_USERNAME and YOUR_REPO_NAME)
git remote add origin https://github.com/YOUR_USERNAME/money-transfer-usecase.git

# Rename branch if needed (optional, if GitHub default is 'main')
git branch -M main

# Push to GitHub
git push -u origin master
# OR if using 'main' branch:
# git push -u origin main
```

## Step 3: Verify

After pushing:
1. Visit https://github.com/YOUR_USERNAME/money-transfer-usecase
2. Verify all files are present:
   - ✓ contracts/repository.go
   - ✓ domain/account.go
   - ✓ repo/account_repo.go
   - ✓ usecases/transfer/interactor.go
   - ✓ usecases/transfer/interactor_test.go
   - ✓ REVIEW.md
   - ✓ ANSWERS.md
   - ✓ README.md
   - ✓ go.mod
   - ✓ .gitignore

## Current Repository Status

```
Commit: ae05c06
Branch: master
Status: Clean (all changes committed)

Files (10 files, 1870 insertions):
- .gitignore
- ANSWERS.md
- README.md
- REVIEW.md
- contracts/repository.go
- domain/account.go
- go.mod
- repo/account_repo.go
- usecases/transfer/interactor.go
- usecases/transfer/interactor_test.go
```

## Alternative: Use GitHub CLI

If you have GitHub CLI installed:

```bash
cd /home/boris/Desktop/med

# Authenticate (first time only)
gh auth login

# Create and push in one command
gh repo create money-transfer-usecase --public --source=. --push --remote=origin
```

## SSH Setup (for future pushes)

For passwordless pushing, set up SSH:

```bash
# Check if SSH keys exist
ls ~/.ssh/id_rsa

# If not, generate keys
ssh-keygen -t rsa -b 4096 -f ~/.ssh/id_rsa

# Add to GitHub: https://github.com/settings/ssh

# Switch remote from HTTPS to SSH
git remote set-url origin git@github.com:YOUR_USERNAME/money-transfer-usecase.git
```

## Troubleshooting

**"fatal: could not read Username"**
- Use fine-grained personal access token instead of password
- Or use SSH keys (recommended)

**"remote: Permission denied"**
- Check GitHub credentials
- Verify you have push access to the repository

**"branch 'master' set up to track 'origin/master'"**
- This is normal - means branch is correctly linked to GitHub

## Repository Is Ready!

The local repository is initialized and ready to push. All files are committed and the working tree is clean.

Run `git log --all` locally to see the commit history anytime.
