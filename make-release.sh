#!/bin/bash
# Release process automation script.
# Used to create branch/tag and update VERSION files
# and and trigger release by force pushing changes to the release branch

RED='\033[0;31m'
GREEN='\033[0;32m' # TODO
NC='\033[0m'

# set to 1 to actually trigger changes in the release branch
TRIGGER_RELEASE="false"
FORCE="false"

# Wrapper for git commands
function gitw() {
  if [[ "$1" == "push" ]] && [[ "$TRIGGER_RELEASE" != "true" ]]; then
    echo -e "    ${RED}'git $*'${NC} (dry run)"
  else
    echo -e "    ${GREEN}git $*${NC}"
    git "$@" 2>&1 | sed 's|^|        |g'
    return "${PIPESTATUS[0]}"
  fi
}

usage ()
{
  echo "Usage: $0 --repo [GIT REPO TO EDIT] --version [VERSION TO RELEASE] [OPTIONS]"
  echo "Options:"
  echo "    --help, -h"
  echo "        Show this message."
  echo "    --trigger-release, -t"
  echo "        Push changes to remote repo, triggering release."
  echo "    --yes, -y"
  echo "        Do not prompt for confirmation before triggering release"
  echo "Example: $0 --repo git@github.com:eclipse/che-subproject --version 'v3.2.1' --trigger-release"; echo
}

while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-r'|'--repo') REPO="$2"; shift 1;;
    '-v'|'--version') VERSION="$2"; shift 1;;
    '-t'|'--trigger-release')
      echo "Triggering release."
      TRIGGER_RELEASE="true"; shift 0;;
    '-y'|'--yes')
      FORCE="true"; shift 0;;
    '-h'|'--help'|*) usage; exit 0;;
  esac
  shift 1
done

# Check script prerequisites.
if [[ ! ${VERSION} ]] || [[ ! ${REPO} ]]; then
  usage
  exit 1
fi
if [[ ${VERSION} != "v"* ]]; then
  echo "Version must be prefixed with 'v'"
  exit 1
fi
if [[ "$TRIGGER_RELEASE" = "true" ]] && [[ "$FORCE" != "true" ]]; then
  echo "Trigger release specified. This will *modify* remote repo $REPO:"
  echo "  1. Create branch ${VERSION%.*}.x"
  echo "  2. Increment VERSION and create a commit"
  echo "  3. Create tag $VERSION in the remote repo"
  echo "  4. Force 'release' branch to the newly created commit"
  echo "  5. Open a PR to increment VERSION in the master branch"
  read -p "Continue? (yes/no) " -r
  if [[ ! $REPLY =~ ^[Yy].*$ ]]; then
    exit 1;
  fi
fi
echo "Releasing version $VERSION in $REPO"

MAJOR_VERSION_BUMP=0
CURRENT_MAJOR_VERSION=$(cut -f 1 -d '.' VERSION)
NEW_MAJOR_VERSION=$(echo "$VERSION" | cut -f 1 -d '.')
if [[ "${NEW_MAJOR_VERSION}" != "${CURRENT_MAJOR_VERSION}" ]]; then
  echo "Incrementing major release version."
  MAJOR_VERSION_BUMP=1
fi

# derive branch from version
BASE_BRANCH="master"
BRANCH=${VERSION%.*}.x
CREATE_BRANCH=false
if [[ ${VERSION} == *".0" ]]; then
  CREATE_BRANCH=true
else
  BASE_BRANCH="$BRANCH"
fi
echo "Working in branch $BRANCH"

# work in tmp dir
TMP=$(mktemp -d); pushd "$TMP" > /dev/null || exit 1

# get sources from ${BASE_BRANCH} branch
echo "Check out $REPO to ${TMP}/${REPO##*/}"
gitw clone "$REPO" -q
cd ./* || exit 1

if [[ "$CREATE_BRANCH" = "true" ]]; then
  echo "Create branch $BRANCH"
  if git ls-remote --exit-code --heads origin "$BRANCH" > /dev/null; then
    echo "Error: branch $BRANCH already exists in $REPO"
    exit 1
  fi
  gitw checkout -b "$BRANCH"
  gitw push origin "$BRANCH"
else
  gitw fetch origin && gitw checkout "$BRANCH"
fi

# change VERSION file
echo "Updating VERSION file to $VERSION"
echo "$VERSION" > VERSION

# commit change into branch
COMMIT_MSG="[release] Bump to $VERSION in $BRANCH"
echo "Creating commit in $BRANCH with message: $COMMIT_MSG"
gitw pull origin "$BRANCH"
gitw commit -s -m "$COMMIT_MSG" VERSION
gitw push origin "$BRANCH"

# force release branch to current commit
echo "Updating branch 'release' to point to current HEAD"
gitw branch release -f
gitw push origin release -f

# tag the release
echo "Creating and pushing git tag $VERSION"
gitw checkout "$BRANCH"
gitw tag "$VERSION"
gitw push origin "$VERSION"

# now update ${BASE_BRANCH} to the new snapshot version
echo "Updating SNAPSHOT version in $BASE_BRANCH"
gitw fetch origin "${BASE_BRANCH}":"${BASE_BRANCH}"
gitw checkout "${BASE_BRANCH}"

# change VERSION file + commit change into ${BASE_BRANCH} branch
if [[ "$MAJOR_VERSION_BUMP" = true ]]; then
  NEXT_VERSION="${NEW_MAJOR_VERSION}.1.0-SNAPSHOT"
elif [[ "${BASE_BRANCH}" != "${BRANCH}" ]]; then
  # bump the y digit
  [[ $BRANCH =~ ^(v[0-9]+)\.([0-9]+)\.x ]] && BASE=${BASH_REMATCH[1]}; NEXT=${BASH_REMATCH[2]}; (( NEXT=NEXT+1 )) # for BRANCH=7.10.x, get BASE=7, NEXT=11
  NEXT_VERSION="${BASE}.${NEXT}.0-SNAPSHOT"
else
  # bump the z digit
  [[ $VERSION =~ ^(v[0-9]+)\.([0-9]+)\.([0-9]+) ]] && BASE="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}"; NEXT="${BASH_REMATCH[3]}"; (( NEXT=NEXT+1 )) # for VERSION=7.7.1, get BASE=7.7, NEXT=2
  NEXT_VERSION="${BASE}.${NEXT}-SNAPSHOT"
fi

# change VERSION file
echo "Updating version in VERSION to $NEXT_VERSION"
echo "${NEXT_VERSION}" > VERSION

# commit change into branch
echo "Committing version change in $BASE_BRANCH"
COMMIT_MSG="[release] Bump to ${NEXT_VERSION} in ${BASE_BRANCH}"
gitw pull origin "${BASE_BRANCH}"
gitw commit -s -m "${COMMIT_MSG}" VERSION

PUSH_TRY="$(gitw push origin "${BASE_BRANCH}")"
# shellcheck disable=SC2181
if [[ $? -gt 0 ]] || [[ $PUSH_TRY == *"protected branch hook declined"* ]]; then
  echo "Could not increment SNAPSHOT version - branch $BASE_BRANCH is protected"
  PR_BRANCH=pr-${BASE_BRANCH}-to-${NEXT_VERSION}
  # create pull request for master branch, as branch is restricted
  gitw checkout -b "${PR_BRANCH}"
  gitw push origin "${PR_BRANCH}"
  lastCommitComment="$(git log -1 --pretty=%B)"
  if command -v hub &> /dev/null; then
    hub pull-request -o -f -m "${lastCommitComment}

${lastCommitComment}" -b "${BASE_BRANCH}" -h "${PR_BRANCH}"
  else
    echo "Could not open PR in $REPO: 'hub' not installed"
    echo "Please open a PR from ${PR_BRANCH} to master to bump next version number"
  fi
fi

popd > /dev/null || exit

# cleanup tmp dir
cd /tmp && rm -fr "$TMP"
