# Grammar for Baseball/Softball Plays

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
       | WALK {% (_d) => _d[0] %}
       | HBP {% (_d) => _d[0] %}
       | ERROR_PLAY {% (_d) => _d[0] %}
       | FC_PLAY {% (_d) => _d[0] %}
       | STEAL_PLAY {% (_d) => _d[0] %}
       | CS_PLAY {% (_d) => _d[0] %}
       | WP_PB {% (_d) => _d[0] %}
       | STATE_ASSERTION {% (_d) => _d[0] %}

PITCH -> "ball" {% (_d) => ({ type: 'PITCH', outcome: 'ball' }) %}
       | "ball" _ int {% (_d) => ({ type: 'PITCH', outcome: 'ball' }) %}
       | "strike" {% (_d) => ({ type: 'PITCH', outcome: 'strike' }) %}
       | "strike" _ int {% (_d) => ({ type: 'PITCH', outcome: 'strike' }) %}
       | "foul" {% (_d) => ({ type: 'PITCH', outcome: 'foul' }) %}
       | "foul" _ "ball" {% (_d) => ({ type: 'PITCH', outcome: 'foul' }) %}

BIP -> "single" {% (_d) => ({ type: 'BIP', result: '1B' }) %}
     | "double" {% (_d) => ({ type: 'BIP', result: '2B' }) %}
     | "triple" {% (_d) => ({ type: 'BIP', result: '3B' }) %}
     | "homerun" {% (_d) => ({ type: 'BIP', result: 'HR' }) %}
     | "single" _ "to" _ POSITION {% (_d) => ({ type: 'BIP', result: '1B', pos: _d[4] }) %}
     | "double" _ "to" _ POSITION {% (_d) => ({ type: 'BIP', result: '2B', pos: _d[4] }) %}
     | "triple" _ "to" _ POSITION {% (_d) => ({ type: 'BIP', result: '3B', pos: _d[4] }) %}
     | "homerun" _ "to" _ POSITION {% (_d) => ({ type: 'BIP', result: 'HR', pos: _d[4] }) %}

OUT -> "strikeout" {% (_d) => ({ type: 'OUT', result: 'strikeout' }) %}
     | "strikeout" _ "looking" {% (_d) => ({ type: 'OUT', result: 'strikeout looking' }) %}
     | "strikeout" _ "swinging" {% (_d) => ({ type: 'OUT', result: 'strikeout swinging' }) %}
     | "fly out" {% (_d) => ({ type: 'OUT', result: 'fly out' }) %}
     | "ground out" {% (_d) => ({ type: 'OUT', result: 'ground out' }) %}
     | "fly out" _ "to" _ POSITION {% (_d) => ({ type: 'OUT', result: 'fly out', pos: _d[4] }) %}
     | "ground out" _ "to" _ POSITION {% (_d) => ({ type: 'OUT', result: 'ground out', pos: _d[4] }) %}
     | "ground out" _ int _ "-" _ int {% (_d) => ({ type: 'OUT', result: 'ground out', sequence: _d[2].toString() + '-' + _d[6].toString() }) %}
     | "line out" {% (_d) => ({ type: 'OUT', result: 'line out' }) %}
     | "line out" _ "to" _ POSITION {% (_d) => ({ type: 'OUT', result: 'line out', pos: _d[4] }) %}
     | "pop out" {% (_d) => ({ type: 'OUT', result: 'pop out' }) %}
     | "pop out" _ "to" _ POSITION {% (_d) => ({ type: 'OUT', result: 'pop out', pos: _d[4] }) %}

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

WALK -> "walk" {% (_d) => ({ type: 'WALK' }) %}
      | "base on balls" {% (_d) => ({ type: 'WALK' }) %}
      | "bb" {% (_d) => ({ type: 'WALK' }) %}

HBP -> "hit by pitch" {% (_d) => ({ type: 'HBP' }) %}
     | "hbp" {% (_d) => ({ type: 'HBP' }) %}

ERROR_PLAY -> "safe on error" {% (_d) => ({ type: 'ERROR', pos: null }) %}
            | "reached on error" {% (_d) => ({ type: 'ERROR', pos: null }) %}
            | "safe on error" _ "to" _ POSITION {% (_d) => ({ type: 'ERROR', pos: _d[4] }) %}
            | "reached on error" _ "to" _ POSITION {% (_d) => ({ type: 'ERROR', pos: _d[4] }) %}
            | "error to" _ POSITION {% (_d) => ({ type: 'ERROR', pos: _d[2] }) %}

FC_PLAY -> "fielder's choice" {% (_d) => ({ type: 'FIELDERS_CHOICE', pos: null }) %}
         | "reached on fielder's choice" {% (_d) => ({ type: 'FIELDERS_CHOICE', pos: null }) %}
         | "fielder's choice" _ "to" _ POSITION {% (_d) => ({ type: 'FIELDERS_CHOICE', pos: _d[4] }) %}
         | "reached on fielder's choice" _ "to" _ POSITION {% (_d) => ({ type: 'FIELDERS_CHOICE', pos: _d[4] }) %}
         | "fc" {% (_d) => ({ type: 'FIELDERS_CHOICE', pos: null }) %}

STEAL_PLAY -> "runner steals" _ BASE {% (_d) => ({ type: 'STEAL', base: _d[2] }) %}
            | "stolen base" {% (_d) => ({ type: 'STEAL', base: null }) %}
            | "stolen base" _ BASE {% (_d) => ({ type: 'STEAL', base: _d[2] }) %}

CS_PLAY -> "runner caught stealing" _ BASE {% (_d) => ({ type: 'CAUGHT_STEALING', base: _d[3] }) %}
         | "runner caught stealing" {% (_d) => ({ type: 'CAUGHT_STEALING', base: null }) %}
         | "caught stealing" _ BASE {% (_d) => ({ type: 'CAUGHT_STEALING', base: _d[2] }) %}

WP_PB -> "wild pitch" {% (_d) => ({ type: 'WILD_PITCH' }) %}
       | "passed ball" {% (_d) => ({ type: 'PASSED_BALL' }) %}

STATE_ASSERTION -> "bases loaded" {% (_d) => ({ type: 'STATE_ASSERTION', bases: [1, 2, 3] }) %}
                 | "bases empty" {% (_d) => ({ type: 'STATE_ASSERTION', bases: [] }) %}
                 | ASSERTION_RUNNERS _ "on" _ BASE _ AND_OR_COMMA _ BASE {% (_d) => { const baseToNum = { '1B': 1, '2B': 2, '3B': 3, 'home': 4 }; return { type: 'STATE_ASSERTION', bases: [baseToNum[_d[4]], baseToNum[_d[8]]].sort() }; } %}
                 | ASSERTION_RUNNERS _ "on" _ BASE {% (_d) => { const baseToNum = { '1B': 1, '2B': 2, '3B': 3, 'home': 4 }; return { type: 'STATE_ASSERTION', bases: [baseToNum[_d[4]]] }; } %}

ASSERTION_RUNNERS -> "runner" | "runners"
AND_OR_COMMA -> "and" | ","

POSITION -> "pitcher" {% (_d) => '1' %}
          | "catcher" {% (_d) => '2' %}
          | "first baseman" {% (_d) => '3' %}
          | "first" {% (_d) => '3' %}
          | "second baseman" {% (_d) => '4' %}
          | "second" {% (_d) => '4' %}
          | "third baseman" {% (_d) => '5' %}
          | "third" {% (_d) => '5' %}
          | "shortstop" {% (_d) => '6' %}
          | "short" {% (_d) => '6' %}
          | "left fielder" {% (_d) => '7' %}
          | "left field" {% (_d) => '7' %}
          | "left" {% (_d) => '7' %}
          | "center fielder" {% (_d) => '8' %}
          | "center field" {% (_d) => '8' %}
          | "center" {% (_d) => '8' %}
          | "right fielder" {% (_d) => '9' %}
          | "right field" {% (_d) => '9' %}
          | "right" {% (_d) => '9' %}
          | [1-9] {% (_d) => _d[0] %}

DIGIT -> [0-9] {% (_d) => _d[0] %}

int -> DIGIT {% (_d) => parseInt(_d[0]) %}
     | int DIGIT {% (_d) => parseInt(_d[0].toString() + _d[1]) %}

# Improved Whitespace: strictly non-null repeat
_ -> null {% (_d) => null %}
   | _ wschar {% (_d) => null %}

wschar -> [ \t\n\r] {% (_d) => null %}
