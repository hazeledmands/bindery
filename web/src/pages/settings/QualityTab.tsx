import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api, QualityProfile } from '../../api/client'

export default function QualityTab() {
  const { t } = useTranslation()
  const [qualityProfiles, setQualityProfiles] = useState<QualityProfile[]>([])

  useEffect(() => {
    api.listQualityProfiles().then(setQualityProfiles).catch(console.error)
  }, [])

  return (
    <div>
      <h3 className="text-lg font-semibold mb-4">{t('settings.quality.heading')}</h3>
      {qualityProfiles.length === 0 ? (
        <p className="text-slate-600 dark:text-zinc-500 text-sm">{t('settings.quality.empty')}</p>
      ) : (
        <div className="space-y-3">
          {qualityProfiles.map(p => (
            <div key={p.id} className="p-4 border border-slate-200 dark:border-zinc-800 rounded-lg bg-slate-100 dark:bg-zinc-900">
              <div className="flex items-center justify-between mb-2">
                <h4 className="font-medium text-sm">{p.name}</h4>
                <div className="flex items-center gap-3 text-xs text-slate-600 dark:text-zinc-500">
                  <span>{t('settings.quality.cutoff')} <span className="text-slate-700 dark:text-zinc-300">{p.cutoff}</span></span>
                  {p.upgradeAllowed && <span className="text-emerald-400">{t('settings.quality.upgradesAllowed')}</span>}
                </div>
              </div>
              {p.items && p.items.length > 0 && (
                <div className="flex flex-wrap gap-1.5 mt-2">
                  {p.items.map((item, i) => (
                    <span key={i} className={`text-[10px] px-2 py-0.5 rounded ${item.allowed ? 'bg-emerald-500/20 text-emerald-400' : 'bg-slate-200 dark:bg-zinc-800 text-slate-500 dark:text-zinc-600'}`}>
                      {item.quality}
                    </span>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
