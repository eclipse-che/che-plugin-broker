#!/bin/bash
# Release process automation script.
# Used to create branch/tag and update VERSION files
# and and trigger release by force pushing changes to the release branch

# set to 1 to actually trigger changes in the release branch
TRIGGER_RELEASE=0
NOCOMMIT=0
MAJOR_VERSION_BUMP=0

usage ()
{
  echo "Usage: $0 --repo [GIT REPO TO EDIT] --version [VERSION TO RELEASE] [OPTIONS]"
  echo "Options:"
  echo "    --help, -h"
  echo "        Show this message."
  echo "    --no-commit, -n"
  echo "        Do not commit changes."
  echo "    --trigger-release, -t"
  echo "        Push changes to remote repo, triggering release."
  echo "Example: $0 --repo git@github.com:eclipse/che-subproject --version 'v3.2.1' --trigger-release"; echo
}

while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-r'|'--repo') REPO="$2"; shift 1;;
    '-v'|'--version') VERSION="$2"; shift 1;;
    '-t'|'--trigger-release')
      if [[ ! ${NOCOMMIT} -eq 0 ]]; then
        echo "Only one of --trigger-release and --no-commit may be specified"
        exit 1
      fi
      TRIGGER_RELEASE=1; shift 0;;
    '-n'|'--no-commit')
      if [[ ! ${NOCOMMIT} -eq 0 ]]; then
        echo "Only one of --trigger-release and --no-commit may be specified"
        exit 1
      fi
      NOCOMMIT=1; shift 0;;
    '-h'|'--help'|*) usage; exit 0;;
  esac
  shift 1
done

if [[ ! ${VERSION} ]] || [[ ! ${REPO} ]]; then
  usage
  exit 1
fi

if [[ ${VERSION} != "v"* ]]; then
  echo "Version must be prefixed with 'v'"
  exit 1
fi

CURRENT_MAJOR_VERSION=$(cut -f 1 -d '.' VERSION)
NEW_MAJOR_VERSION=$(echo "$VERSION" | cut -f 1 -d '.')
if [[ "${NEW_MAJOR_VERSION}" != "${CURRENT_MAJOR_VERSION}" ]]; then
  MAJOR_VERSION_BUMP=1
fi

# derive branch from version
BRANCH=${VERSION%.*}.x

# if doing a .0 release, use master; if doing a .z release, use $BRANCH
if [[ ${VERSION} == *".0" ]]; then
  BASEBRANCH="master"
else
  BASEBRANCH="${BRANCH}"
fi

# work in tmp dir
TMP=$(mktemp -d); pushd "$TMP" > /dev/null || exit 1

# get sources from ${BASEBRANCH} branch
echo "Check out ${REPO} to ${TMP}/${REPO##*/}"
git clone "${REPO}" -q
cd "${REPO##*/}" || exit 1
git fetch origin "${BASEBRANCH}":"${BASEBRANCH}"
git checkout "${BASEBRANCH}"

# create new branch off ${BASEBRANCH} (or check out latest commits if branch already exists), then push to origin
if [[ "${BASEBRANCH}" != "${BRANCH}" ]]; then
  git branch "${BRANCH}" || git checkout "${BRANCH}" && git pull origin "${BRANCH}"
  git push origin "${BRANCH}"
  git fetch origin "${BRANCH}:${BRANCH}"
  git checkout "${BRANCH}"
fi

# change VERSION file
echo "${VERSION}" > VERSION

# commit change into branch
if [[ ${NOCOMMIT} -eq 0 ]]; then
  COMMIT_MSG="[release] Bump to ${VERSION} in ${BRANCH}"
  git commit -s -m "${COMMIT_MSG}" VERSION
  git pull origin "${BRANCH}"
  git push origin "${BRANCH}"
fi

if [[ ${TRIGGER_RELEASE} -eq 1 ]]; then
  # push new branch to release branch to trigger CI build
  git fetch origin "${BRANCH}:${BRANCH}"
  git checkout "${BRANCH}"
  git branch release -f
  git push origin release -f

  # tag the release
  git checkout "${BRANCH}"
  git tag "${VERSION}"
  git push origin "${VERSION}"
fi

# now update ${BASEBRANCH} to the new snapshot version
git fetch origin "${BASEBRANCH}":"${BASEBRANCH}"
git checkout "${BASEBRANCH}"

# change VERSION file + commit change into ${BASEBRANCH} branch
if [[ ${MAJOR_VERSION_BUMP} -eq 1 ]]; then
  NEXTVERSION="${NEW_MAJOR_VERSION}.1.0-SNAPSHOT"
elif [[ "${BASEBRANCH}" != "${BRANCH}" ]]; then
  # bump the y digit
  [[ $BRANCH =~ ^(v[0-9]+)\.([0-9]+)\.x ]] && BASE=${BASH_REMATCH[1]}; NEXT=${BASH_REMATCH[2]}; (( NEXT=NEXT+1 )) # for BRANCH=7.10.x, get BASE=7, NEXT=11
  NEXTVERSION="${BASE}.${NEXT}.0-SNAPSHOT"
else
  # bump the z digit
  [[ $VERSION =~ ^(v[0-9]+)\.([0-9]+)\.([0-9]+) ]] && BASE="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}"; NEXT="${BASH_REMATCH[3]}"; (( NEXT=NEXT+1 )) # for VERSION=7.7.1, get BASE=7.7, NEXT=2
  NEXTVERSION="${BASE}.${NEXT}-SNAPSHOT"
fi

# change VERSION file
echo "${NEXTVERSION}" > VERSION

if [[ ${NOCOMMIT} -eq 0 ]]; then
  BRANCH=${BASEBRANCH}
  # commit change into branch
  COMMIT_MSG="[release] Bump to ${NEXTVERSION} in ${BRANCH}"
  git commit -s -m "${COMMIT_MSG}" VERSION
  git pull origin "${BRANCH}"

  PUSH_TRY="$(git push origin "${BRANCH}")"
  # shellcheck disable=SC2181
  if [[ $? -gt 0 ]] || [[ $PUSH_TRY == *"protected branch hook declined"* ]]; then
  PR_BRANCH=pr-master-to-${NEXTVERSION}
    # create pull request for master branch, as branch is restricted
    git branch "${PR_BRANCH}"
    git checkout "${PR_BRANCH}"
    git pull origin "${PR_BRANCH}"
    git push origin "${PR_BRANCH}"
    lastCommitComment="$(git log -1 --pretty=%B)"
    hub pull-request -o -f -m "${lastCommitComment}

${lastCommitComment}" -b "${BRANCH}" -h "${PR_BRANCH}"
  fi
fi

popd > /dev/null || exit

# cleanup tmp dir
cd /tmp && rm -fr "$TMP"
