import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api, RootFolder } from '../../api/client'
import { inputCls } from './formStyles'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

export default function RootFoldersTab() {
  const { t } = useTranslation()
  const [rootFolders, setRootFolders] = useState<RootFolder[]>([])
  const [newFolderPath, setNewFolderPath] = useState('')
  const [folderError, setFolderError] = useState('')

  useEffect(() => {
    api.listRootFolders().then(setRootFolders).catch(console.error)
  }, [])

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h3 className="text-lg font-semibold">{t('settings.rootfolders.heading')}</h3>
      </div>
      <p className="text-sm text-slate-600 dark:text-zinc-400 mb-4">
        {t('settings.rootfolders.description')} (<code className="font-mono bg-slate-200 dark:bg-zinc-800 px-1 rounded text-xs">BINDERY_LIBRARY_DIR</code>).
      </p>

      {rootFolders.length > 0 && (
        <div className="space-y-2 mb-6">
          {rootFolders.map(rf => (
            <div key={rf.id} className="flex items-center justify-between p-4 border border-slate-200 dark:border-zinc-800 rounded-lg bg-slate-100 dark:bg-zinc-900">
              <div className="min-w-0">
                <p className="font-mono text-sm truncate">{rf.path}</p>
                <p className="text-xs text-slate-500 dark:text-zinc-500 mt-0.5">{t('settings.rootfolders.free', { size: formatBytes(rf.freeSpace) })}</p>
              </div>
              <button
                onClick={async () => {
                  await api.deleteRootFolder(rf.id)
                  setRootFolders(rootFolders.filter(f => f.id !== rf.id))
                }}
                className="ml-4 px-3 py-1 text-xs text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded border border-red-200 dark:border-red-800 flex-shrink-0"
              >
                {t('common.remove')}
              </button>
            </div>
          ))}
        </div>
      )}

      <form
        onSubmit={async e => {
          e.preventDefault()
          setFolderError('')
          try {
            const created = await api.addRootFolder(newFolderPath.trim())
            setRootFolders([...rootFolders, created])
            setNewFolderPath('')
          } catch (err: unknown) {
            setFolderError(err instanceof Error ? err.message : 'Failed to add folder')
          }
        }}
        className="flex gap-2 items-start"
      >
        <div className="flex-1">
          <input
            value={newFolderPath}
            onChange={e => { setNewFolderPath(e.target.value); setFolderError('') }}
            placeholder={t('settings.rootfolders.addPlaceholder')}
            className={inputCls}
          />
          {folderError && <p className="text-xs text-red-500 mt-1">{folderError}</p>}
        </div>
        <button
          type="submit"
          disabled={!newFolderPath.trim()}
          className="px-3 py-2 bg-emerald-600 hover:bg-emerald-500 rounded text-sm font-medium disabled:opacity-50 flex-shrink-0"
        >
          {t('settings.rootfolders.addButton')}
        </button>
      </form>
    </div>
  )
}
