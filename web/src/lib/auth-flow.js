export function getPostLoginPath(user) {
  return user?.terms_accepted ? "/console/dashboard" : "/accept-terms";
}

export function getConsoleAuthRedirect(error) {
  if (!error) {
    return null;
  }
  if (error.status === 401) {
    return "/login";
  }
  if (error.code === "auth.terms_required") {
    return "/accept-terms";
  }
  return null;
}
