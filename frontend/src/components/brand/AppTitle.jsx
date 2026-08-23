export default function AppTitle({ className = '' }) {
  const classes = ['app-title', className].filter(Boolean).join(' ')
  return <h1 className={classes}>JoinQuest</h1>
}
