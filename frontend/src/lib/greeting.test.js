import { describe, it, expect } from 'vitest'
import { homeGreetingLine, timeOfDayGreeting } from './greeting'

function at(hour) {
  return new Date(2026, 0, 1, hour, 30)
}

describe('timeOfDayGreeting', () => {
  it('greets the morning before noon', () => {
    expect(timeOfDayGreeting(at(0))).toBe('Good morning')
    expect(timeOfDayGreeting(at(11))).toBe('Good morning')
  })

  it('greets the afternoon from noon until six', () => {
    expect(timeOfDayGreeting(at(12))).toBe('Good afternoon')
    expect(timeOfDayGreeting(at(17))).toBe('Good afternoon')
  })

  it('greets the evening from six onwards', () => {
    expect(timeOfDayGreeting(at(18))).toBe('Good evening')
    expect(timeOfDayGreeting(at(23))).toBe('Good evening')
  })
})

describe('homeGreetingLine', () => {
  it('appends the name when there is one', () => {
    expect(homeGreetingLine('FrostFox2841', at(13))).toBe('Good afternoon, FrostFox2841')
  })

  it('leaves off the comma when there is no name', () => {
    expect(homeGreetingLine('', at(13))).toBe('Good afternoon')
    expect(homeGreetingLine(undefined, at(9))).toBe('Good morning')
    expect(homeGreetingLine('   ', at(20))).toBe('Good evening')
  })
})
