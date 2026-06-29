const ACTION_BY_STEP = {
  api_key: {
    href: "/console/api-keys",
    label: "Create key",
  },
  first_call: {
    href: "/console/dashboard#quick-start",
    label: "View curl",
  },
};

export function getActivationStepAction(step) {
  if (!step || step.status === "completed") {
    return null;
  }
  return ACTION_BY_STEP[step.key] || null;
}

export function getNextActivationAction(steps = []) {
  for (const step of steps) {
    const action = getActivationStepAction(step);
    if (action) {
      return action;
    }
  }
  return null;
}
