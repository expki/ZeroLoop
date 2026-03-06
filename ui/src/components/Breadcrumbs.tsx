import { useMemo } from 'react'

interface BreadcrumbsProps {
  filePath: string | null
}

export default function Breadcrumbs({ filePath }: BreadcrumbsProps) {
  const segments = useMemo(() => {
    if (!filePath) return []
    return filePath.split('/')
  }, [filePath])

  if (!filePath) return null

  return (
    <div className="breadcrumbs">
      {segments.map((segment, index) => (
        <span key={index} className="breadcrumb-item">
          {index > 0 && <span className="breadcrumb-separator">/</span>}
          <span className={index === segments.length - 1 ? 'breadcrumb-current' : 'breadcrumb-parent'}>
            {segment}
          </span>
        </span>
      ))}
    </div>
  )
}
