/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { useDepartments, useUpdateDepartments } from '../hooks/use-departments'
import { DEFAULT_DEPARTMENT } from '../lib/departments'

type DepartmentsManageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

/**
 * 管理部门清单。增删在本地草稿上进行，保存时整体写入后端设置项。
 * 「公共部门」不可删除；删除其他部门不会改动已分配到该部门的用户，
 * 管理员可在用户编辑页把用户改到其他部门。
 */
export function DepartmentsManageDialog({
  open,
  onOpenChange,
}: DepartmentsManageDialogProps) {
  const { t } = useTranslation()
  const { data: serverDepartments } = useDepartments()
  const updateMutation = useUpdateDepartments()
  const [draft, setDraft] = useState<string[]>([])
  const [newName, setNewName] = useState('')

  // 弹窗打开时用最新服务器数据重新播种本地草稿。
  useEffect(() => {
    if (open && serverDepartments) {
      setDraft(serverDepartments)
      setNewName('')
    }
  }, [open, serverDepartments])

  const trimmedNewName = newName.trim()
  const isDuplicate = draft.some(
    (department) => department.trim() === trimmedNewName
  )
  const canAdd = trimmedNewName.length > 0 && !isDuplicate

  const handleAdd = () => {
    if (!canAdd) return
    setDraft((current) => [...current, trimmedNewName])
    setNewName('')
  }

  const handleSave = () => {
    const next = draft.map((department) => department.trim()).filter(Boolean)
    // 默认部门始终保留，避免保存后部门清单缺少兜底部门。
    if (!next.includes(DEFAULT_DEPARTMENT)) {
      next.unshift(DEFAULT_DEPARTMENT)
    }
    updateMutation.mutate(next, {
      onSuccess: () => onOpenChange(false),
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('Manage Departments')}</DialogTitle>
          <DialogDescription>
            {t(
              'Add or remove departments. Users are assigned to a department from the user page.'
            )}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-3'>
          <div className='flex flex-wrap gap-2'>
            {draft.map((department, index) => (
              <Badge key={department} variant='outline' className='gap-1 py-1'>
                {department}
                <Button
                  variant='ghost'
                  size='icon-sm'
                  aria-label={t('Remove {{department}}', { department })}
                  disabled={department === DEFAULT_DEPARTMENT}
                  onClick={() =>
                    setDraft((current) =>
                      current.filter((_, position) => position !== index)
                    )
                  }
                >
                  <Trash2 />
                </Button>
              </Badge>
            ))}
            {draft.length === 0 && (
              <p className='text-muted-foreground text-sm'>
                {t('No departments configured.')}
              </p>
            )}
          </div>

          <div className='space-y-2'>
            <Label htmlFor='new-department'>{t('New department')}</Label>
            <div className='flex gap-2'>
              <Input
                id='new-department'
                value={newName}
                placeholder={t('Enter department name')}
                onChange={(event) => setNewName(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault()
                    handleAdd()
                  }
                }}
              />
              <Button
                type='button'
                variant='outline'
                disabled={!canAdd}
                onClick={handleAdd}
              >
                {t('Add')}
              </Button>
            </div>
          </div>

          <Alert>
            <AlertDescription className='text-xs'>
              {t(
                'The default department cannot be removed. Removing a department does not change users already assigned to it; update them from the user page.'
              )}
            </AlertDescription>
          </Alert>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button disabled={updateMutation.isPending} onClick={handleSave}>
            {updateMutation.isPending ? t('Saving...') : t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
