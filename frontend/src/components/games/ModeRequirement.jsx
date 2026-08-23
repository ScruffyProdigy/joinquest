export default function ModeRequirement({ requirement }) {
  if (!requirement) {
    return null
  }

  if (requirement.__typename === 'RequirementGroup') {
    const joiner = requirement.operator === 'ANY' ? 'or' : 'and'
    return (
      <span className="mode-requirement mode-requirement--group">
        {(requirement.children ?? []).map((child, index) => (
          <span key={`${child.label}-${index}`}>
            {index > 0 ? <span className="mode-requirement__joiner"> {joiner} </span> : null}
            <ModeRequirement requirement={child} />
          </span>
        ))}
      </span>
    )
  }

  return (
    <span className="mode-requirement mode-requirement--leaf">
      {requirement.current}/{requirement.target} {requirement.label}
    </span>
  )
}
