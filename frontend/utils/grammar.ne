# Grammar for Baseball Plays

# Custom Whitespace and Numbers to avoid builtin lint issues with 'd'
# We use manually expanded repeat rules to avoid ebnf arrpush(d) lint errors

START -> _ PLAY _ {% (_d) => _d[1] %}

PLAY -> EVENT {% (_d) => [_d[0]] %}
      | PLAY DELIMITER EVENT {% (_d) => [..._d[0], _d[2]] %}

DELIMITER -> _ "," _ {% (_d) => null %}
           | _ "and" _ {% (_d) => null %}
           | _ "then" _ {% (_d) => null %}

EVENT -> PITCH {% (_d) => _d[0] %}
       | BIP {% (_d) => _d[0] %}
       | OUT {% (_d) => _d[0] %}
       | RUNNER_ACTION {% (_d) => _d[0] %}
       | SUB {% (_d) => _d[0] %}

PITCH -> "ball" {% (_d) => ({ type: 'PITCH', outcome: 'ball' }) %}
       | "strike" {% (_d) => ({ type: 'PITCH', outcome: 'strike' }) %}
       | "foul" {% (_d) => ({ type: 'PITCH', outcome: 'foul' }) %}

BIP -> "single" {% (_d) => ({ type: 'BIP', result: '1B' }) %}
     | "double" {% (_d) => ({ type: 'BIP', result: '2B' }) %}
     | "triple" {% (_d) => ({ type: 'BIP', result: '3B' }) %}
     | "homerun" {% (_d) => ({ type: 'BIP', result: 'HR' }) %}

OUT -> "strikeout" {% (_d) => ({ type: 'OUT', result: 'strikeout' }) %}
     | "fly out" {% (_d) => ({ type: 'OUT', result: 'fly out' }) %}
     | "ground out" {% (_d) => ({ type: 'OUT', result: 'ground out' }) %}

RUNNER_ACTION -> "runner" _ "to" _ BASE {% (_d) => ({ type: 'RUNNER_ADVANCE', base: _d[4] }) %}
               | "runner" _ "scores" {% (_d) => ({ type: 'RUNNER_ADVANCE', base: 'home' }) %}

BASE -> "first" {% (_d) => '1B' %}
      | "second" {% (_d) => '2B' %}
      | "third" {% (_d) => '3B' %}
      | "home" {% (_d) => 'home' %}

SUB -> "pinch runner" _ PLAYER_REF {% (_d) => ({ type: 'SUBSTITUTION', player: _d[2], replaced: null }) %}
     | "pinch runner" _ PLAYER_REF _ "for" _ PLAYER_REF {% (_d) => ({ type: 'SUBSTITUTION', player: _d[2], replaced: _d[6] }) %}
     | "pitching change" _ PLAYER_REF {% (_d) => ({ type: 'PITCHER_UPDATE', player: _d[2], replaced: null }) %}
     | "pitching change" _ PLAYER_REF _ "for" _ PLAYER_REF {% (_d) => ({ type: 'PITCHER_UPDATE', player: _d[2], replaced: _d[6] }) %}

PLAYER_REF -> "number" _ int {% (_d) => ({ jersey: _d[2] }) %}
            | int {% (_d) => ({ jersey: _d[0] }) %}

DIGIT -> [0-9] {% (_d) => _d[0] %}

int -> DIGIT {% (_d) => parseInt(_d[0]) %}
     | int DIGIT {% (_d) => parseInt(_d[0].toString() + _d[1]) %}

# Improved Whitespace: strictly non-null repeat
_ -> null {% (_d) => null %}
   | _ wschar {% (_d) => null %}

wschar -> [ \t\n\r] {% (_d) => null %}
