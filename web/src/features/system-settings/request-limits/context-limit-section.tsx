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
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { t } from 'i18next'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeJsonParse } from '../utils/json-parser'
import {
  canonicalModelLimitJson,
  formatModelLimitLines,
  isValidModelLimitText,
  parseModelLimitLines,
  type ModelLimitMap,
} from './context-limit-utils'

const modelLimitValidationMessage = t(
  'Each line must be in the format model=limit with a non-negative integer'
)

const contextLimitSchema = z.object({
  context_limit_setting: z.object({
    enabled: z.boolean(),
    default_max_prompt_tokens: z.number().int().min(0),
    model_max_prompt_tokens: z.string().refine(isValidModelLimitText, {
      message: modelLimitValidationMessage,
    }),
  }),
})

type ContextLimitFormValues = z.output<typeof contextLimitSchema>
type ContextLimitFormInput = z.input<typeof contextLimitSchema>

type NormalizedContextLimitValues = {
  'context_limit_setting.enabled': boolean
  'context_limit_setting.default_max_prompt_tokens': number
  'context_limit_setting.model_max_prompt_tokens': string
}

type ContextLimitSectionProps = {
  defaultValues: NormalizedContextLimitValues
}

const buildFormDefaults = (
  defaults: ContextLimitSectionProps['defaultValues']
): ContextLimitFormInput => ({
  context_limit_setting: {
    enabled: defaults['context_limit_setting.enabled'],
    default_max_prompt_tokens:
      defaults['context_limit_setting.default_max_prompt_tokens'],
    model_max_prompt_tokens: formatModelLimitLines(
      safeJsonParse<ModelLimitMap>(
        defaults['context_limit_setting.model_max_prompt_tokens'],
        { fallback: {} }
      )
    ),
  },
})

const normalizeDefaults = (
  defaults: ContextLimitSectionProps['defaultValues']
): NormalizedContextLimitValues => ({
  'context_limit_setting.enabled': defaults['context_limit_setting.enabled'],
  'context_limit_setting.default_max_prompt_tokens':
    defaults['context_limit_setting.default_max_prompt_tokens'],
  'context_limit_setting.model_max_prompt_tokens': canonicalModelLimitJson(
    safeJsonParse<ModelLimitMap>(
      defaults['context_limit_setting.model_max_prompt_tokens'],
      { fallback: {} }
    )
  ),
})

const normalizeFormValues = (
  values: ContextLimitFormValues
): NormalizedContextLimitValues => ({
  'context_limit_setting.enabled':
    values.context_limit_setting.enabled,
  'context_limit_setting.default_max_prompt_tokens':
    values.context_limit_setting.default_max_prompt_tokens,
  'context_limit_setting.model_max_prompt_tokens':
    canonicalModelLimitJson(
      parseModelLimitLines(values.context_limit_setting.model_max_prompt_tokens)
    ),
})

export function ContextLimitSection({ defaultValues }: ContextLimitSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const baselineRef = useRef<NormalizedContextLimitValues>(
    normalizeDefaults(defaultValues)
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<ContextLimitFormInput, unknown, ContextLimitFormValues>({
    resolver: zodResolver(contextLimitSchema),
    mode: 'onChange',
    defaultValues: formDefaults,
  })

  useEffect(() => {
    baselineRef.current = normalizeDefaults(defaultValues)
    form.reset(buildFormDefaults(defaultValues))
  }, [defaultValues, form])

  const onSubmit = async (data: ContextLimitFormValues) => {
    const normalized = normalizeFormValues(data)
    const updates = (
      Object.keys(normalized) as Array<keyof NormalizedContextLimitValues>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of updates) {
      await updateOption.mutateAsync({ key, value: normalized[key] })
    }

    baselineRef.current = normalized
  }

  return (
    <SettingsSection title={t('Context Limits')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            isSaveDisabled={!form.formState.isValid}
            saveLabel='Save context limits'
          />
          <FormField
            control={form.control}
            name='context_limit_setting.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable context limits')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Reject requests whose estimated prompt tokens exceed the model limit, before forwarding upstream'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='context_limit_setting.default_max_prompt_tokens'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Default max prompt tokens')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={0}
                    step={1}
                    {...field}
                    onChange={(e) =>
                      field.onChange(
                        Number.parseInt(e.target.value) || 0
                      )
                    }
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Applies to models without an explicit per-model limit. 0 disables the limit.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='context_limit_setting.model_max_prompt_tokens'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Per-model max prompt tokens')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={5}
                    className='font-mono'
                    placeholder='glm-4.7=210000'
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'One rule per line: model=token limit. A per-model 0 disables the limit for that model.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
