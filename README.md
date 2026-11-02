# Lynx → JavaScript

Компилятор-транспилятор языка **Lynx** в JavaScript, написанный на Go.
Lynx — строго типизированный язык; компилятор проверяет типы до генерации кода
и сообщает об ошибках с указанием файла, строки, столбца и фрагмента исходника.

## Возможности

- типы `Number`, `String`, `Array`, `Any`, `Unknown`, `Null` и nullable-типы
  вида `String?`;
- константы `const` (синоним `val`) и изменяемые переменные `var` с
  необязательной аннотацией типа;
- функции `fun`: типизированные параметры, значения по умолчанию, nullable- и
  вариадические параметры, необязательный тип возврата;
- анонимные функции (лямбды), типы функций и замыкания;
- вызовы с позиционными и именованными аргументами;
- `return` с выражением и без него;
- строгая проверка типов при объявлении, присваивании, вызове и возврате;
- арифметика, массивы, индексация, `.length`, блоки и комментарии.

Язык остаётся намеренно компактным: в нём пока нет условий, циклов, объектов и
импортов.

## Установка и сборка

Требуется Go 1.22 или новее.

```bash
go build -o bin/lynxc ./cmd/lynxc
```

Или через `make`:

```bash
make build
```

## Использование

```bash
# Вывести JavaScript в stdout
./bin/lynxc examples/functions.lynx

# Записать результат в файл
./bin/lynxc -o examples/functions.js examples/functions.lynx

# Только проверить типы, без генерации
./bin/lynxc -check examples/functions.lynx

# Прочитать исходник из stdin
cat examples/functions.lynx | ./bin/lynxc -
```

Опции CLI:

```
-o <file>   записать JavaScript в файл (по умолчанию stdout)
-check      только проверка типов
-version    версия компилятора
```

Опции можно указывать как до, так и после имени файла.

## Синтаксис языка

Программа — это последовательность объявлений переменных, функций, вызовов и
`return` (внутри функций). Точка с запятой в конце инструкции необязательна.

### Типы данных

| Тип Lynx | Поведение |
| --- | --- |
| `Number` | числовые литералы и арифметические выражения |
| `String` | строковые литералы, конкатенация |
| `Array` | массивы; внутренне хранится тип элементов |
| `Any` | динамическое значение, проверки ослаблены |
| `Unknown` | значение неизвестного типа |
| `Null` | тип литерала `null` |
| `T?` | значение типа `T` или `null` |
| `(T, ...) -> R` | тип функции: параметры и тип возврата |

Аннотация типа записывается после двоеточия и стирается при генерации
JavaScript. `Int` — синоним `Number`:

```lynx
const count: Number = 10
var name: String = "Lynx"
const items: Array = [1, 2, 3]
const missing: Null = null
var nickname: String? = null
```

`T?` — это именно nullable-тип, а не необязательный параметр: значение
по-прежнему нужно передать, если у параметра нет значения по умолчанию.

Правила проверки типов:

- если аннотация не указана, тип выводится из начального значения;
- `Number`, `String` и `Array` несовместимы между собой;
- `Any` можно присвоить куда угодно и получить откуда угодно;
- `Unknown` можно присвоить только в `Unknown` или `Any`; применять к нему
  операторы, индексацию и `.length` нельзя (сужение типа пока не
  поддерживается);
- `null` можно присвоить только в nullable-тип, `Any` или `Unknown`;
- к nullable-значениям нельзя применять операторы, индексацию и `.length`:
  проверки на `null` и сужения типов в языке пока нет;
- в исходном коде тип элемента массива не указывается — пишется просто
  `Array`; запись `Array<Number>` используется только во внутренней
  диагностике;
- `Array` без параметра ведёт себя как `Array<Any>`;
- смешанный массив, например `[1, "two"]`, выводится как `Array<Any>`;
- массив с `null`, например `[1, null]`, выводится как `Array<Number?>`;
- индекс массива должен быть `Number`, `.length` возвращает `Number`.

Массивы создаются литералами, индексируются квадратными скобками и имеют
свойство `.length`:

```lynx
const numbers = [1, 2, 3]
const first: Number = numbers[0]
const size: Number = numbers.length
```

### Переменные

Константа объявляется через `const` и требует начального значения. Вместо
`const` можно писать `val`:

```lynx
const a = 10
const typedConstant: Number = 10
```

Изменяемая переменная объявляется через `var`; начальное значение можно
опустить, если указан тип:

```lynx
var b = 10
var typedVariable: Number = 10
var pending: Number

typedVariable = typedVariable + 1
pending = 5
```

Синтаксис:

```
const имя = выражение
const имя: Тип = выражение
var имя = выражение
var имя: Тип = выражение
var имя: Тип
```

Правила:

- `const` без начального значения — ошибка;
- `var` без значения обязана иметь аннотацию типа, иначе тип вывести не из
  чего;
- тип `var` фиксируется при объявлении и не меняется при присваивании;
- переприсваивание `const` запрещено;
- повторное объявление имени в одной области видимости запрещено;
- блоки `{ ... }` создают вложенную область видимости.

Исходник:

```lynx
const a = 10;
const typedConstant: Number = 10;

var b = 10;
var typedVariable: Number = 10;
typedVariable = typedVariable + 1;

const greeting: String = "Hello, Lynx!";
var dynamic: Any = 10;
dynamic = "now a string";

const nothing: Null = null;
var maybe: String? = null;
maybe = "defined";
```

Результат:

```js
const a = 10;
const typedConstant = 10;
let b = 10;
let typedVariable = 10;
typedVariable = typedVariable + 1;
const greeting = "Hello, Lynx!";
let dynamic = 10;
dynamic = "now a string";
const nothing = null;
let maybe = null;
maybe = "defined";
```

Аннотации типов стираются: JavaScript-код не содержит типов Lynx.
`const` превращается в `const`, `var` — в `let`.

### Функции

Объявление функции:

```lynx
fun имя(параметр: Тип, параметр: Тип = значение): ТипВозврата {
  ...
  return выражение
}
```

Пример:

```lynx
fun example(b: String? = null, a: Number = 10): String? {
  return b
}

example(b = "test", a = 10)
example("test", 10)
example()
```

Результат:

```js
function example(b = null, a = 10) {
  return b;
}
example("test", 10);
example("test", 10);
example();
```

Параметры:

- каждый параметр обязан иметь аннотацию типа: `a: Number`;
- значение по умолчанию задаётся через `=`: `a: Number = 10`;
- значение по умолчанию проверяется на совместимость с типом параметра и может
  ссылаться на ранее объявленные параметры;
- nullable-параметр: `b: String?`, его значением по умолчанию может быть `null`;
- вариадический параметр записывается как `numbers: Number...`; он должен быть
  последним, может быть только один, не может иметь значения по умолчанию и не
  может быть передан по имени;
- внутри тела вариадический параметр имеет тип `Array<Number>`:

```lynx
fun first(numbers: Number...): Number {
  return numbers[0]
}

const value = first(1, 2, 3, 4)
```

Возврат значения:

- `fun f(): Number` — функция обязана вернуть значение совместимого типа;
- `return выражение` возвращает значение;
- `return` без выражения допустим только в функции без типа возврата: в
  функции с типом возврата это ошибка;
- функция без аннотации возврата считается `void`: `return выражение` в ней
  запрещён;
- если тип возврата указан, в теле функции должен быть хотя бы один `return`
  со значением; компилятор проверяет наличие `return`, но не анализирует все
  ветви выполнения.

Вызовы:

```lynx
example("test", 10)          // позиционные аргументы
example(b = "test", a = 10)  // именованные аргументы
example()                    // только параметры со значениями по умолчанию
```

- позиционные аргументы сопоставляются параметрам по порядку;
- именованные аргументы переставляются компилятором в позиционные;
- позиционный аргумент не может идти после именованного;
- один параметр нельзя указать дважды;
- обязательные параметры без значения по умолчанию должны быть переданы;
- если при именованном вызове пропущен параметр со значением по умолчанию,
  в JavaScript передаётся `undefined`, чтобы сработало значение по умолчанию:

```lynx
fun f(a: Number = 1, b: Number = 2): Number {
  return b
}

f(b = 5) // станет f(undefined, 5)
```

Прочее:

- сигнатуры функций собираются до проверки тел, поэтому функции могут вызывать
  друг друга и функции, объявленные ниже; рекурсия поддерживается;
- вложенные именованные функции пока запрещены — объявления `fun` допустимы
  только на верхнем уровне;
- имена функций не должны совпадать с именами переменных и параметров.

### Анонимные функции

Анонимная функция (лямбда) записывается в фигурных скобках: параметры с типами,
затем `->` и выражение, значение которого возвращает функция.

```lynx
fun log(value: Number) {}

val sum = { x: Int, y: Int ->
  x + y
}

log(sum(1, 5))
```

Результат:

```js
function log(value) {
}
const sum = (x, y) => x + y;
log(sum(1, 5));
```

Тип лямбды выводится автоматически и записывается как
`(Number, Number) -> Number`. Тип функции можно указать явно:

```lynx
const sum: (Number, Number) -> Number = {
  x: Number, y: Number -> x + y
}
```

Типы функций разрешены в аннотациях параметров и возвращаемых значений:

```lynx
fun apply(f: (Number) -> Number, value: Number): Number {
  return f(value)
}

const incremented = apply({ n: Number -> n + 1 }, 10)
```

Лямбда без параметров записывается как `{ -> 42 }`. Лямбда может обращаться к
переменным из внешней области видимости (замыкание):

```lynx
const outer = { x: Number -> { y: Number -> x + y } }
const addOne = outer(1)
const result = addOne(41) // 42
```

Ограничения:

- тело лямбды — одно выражение; блоков и `return` внутри лямбды пока нет;
- все параметры лямбды должны иметь аннотацию типа;
- значения по умолчанию и вариадические параметры в лямбдах не поддерживаются;
- тело лямбды должно возвращать значение, поэтому вызов `void`-функции в теле
  запрещён;
- вызывать можно только переменную с типом функции; вызов произвольного
  выражения, например `fs[0](1)`, пока не поддерживается;
- именованные аргументы для функций-значений не поддерживаются.

## Операторы и выражения

Поддерживаются:

- арифметика `+ - * / %`, унарные `+` и `-`, скобки;
- строковая конкатенация через `+`;
- массивы `[1, 2, 3]`, доступ по индексу `a[0]`, свойство `a.length`;
- вызовы функций как выражения (например, в инициализаторе переменной);
- комментарии `// ...` и `/* ... */`.

## Пример диагностики

```
$ ./bin/lynxc bad.lynx
bad.lynx:1:19: error: cannot assign a value of type String to variable "a" of type Number
const a: Number = "x";
                  ^
```

## Точки с запятой

Точки с запятой необязательны. Однако перевод строки не является отдельным
токеном, поэтому пустой `return` перед следующим выражением лучше писать как
`return;` — иначе следующее выражение будет воспринято как возвращаемое
значение.

## Структура проекта

```
cmd/lynxc/          CLI-точка входа
internal/token/     токены и позиции
internal/lexer/     лексер
internal/ast/       узлы синтаксического дерева
internal/parser/    рекурсивный парсер
internal/checker/   строгая проверка типов
internal/codegen/   генерация JavaScript
internal/compiler/  объединение всех этапов
internal/diag/      диагностика с фрагментами исходника
examples/           примеры на Lynx
```

## Тесты

```bash
go test ./...
```

Или:

```bash
make test
```

Тестами покрыты лексер, парсер, проверка типов, генерация кода, форматирование
ошибок, CLI и компиляция примеров из `examples/`.



# Обработка ошибок в языке (рабочее название — *Lang*)

**Версия документа:** 1.0
**Статус:** стабильная спецификация
**Область:** исключения, ошибки, паники, асинхронность, интероп с JavaScript

---

## Оглавление

1. [Философия и основные принципы](#1-философия-и-основные-принципы)
2. [Терминология](#2-терминология)
3. [Типы ошибок](#3-типы-ошибок)
4. [Тип `Result<T, E>`](#4-тип-resultt-e)
5. [Сигнатуры функций: `throws`](#5-сигнатуры-функций-throws)
6. [Наборы ошибок (error sets)](#6-наборы-ошибок-error-sets)
7. [Оператор `?`](#7-оператор-)
8. [Обработка ошибок: `match`, `catch`, `recover`](#8-обработка-ошибок-match-catch-recover)
9. [Контекст ошибок и стек-трейсы](#9-контекст-ошибок-и-стек-трейсы)
10. [Асинхронность](#10-асинхронность)
11. [Паники (`panic!`)](#11-паники-panic)
12. [Тип `Dynamic` и границы приложения](#12-тип-dynamic-и-границы-приложения)
13. [Интероп с JavaScript](#13-интероп-с-javascript)
14. [Компиляция в JavaScript: правила и инварианты](#14-компиляция-в-javascript-правила-и-инварианты)
15. [Стандартная библиотека: модуль `core.result`](#15-стандартная-библиотека-модуль-coreresult)
16. [Стиль и рекомендации](#16-стиль-и-рекомендации)
17. [Антипаттерны](#17-антипаттерны)
18. [Приложения](#18-приложения)

---

## 1. Философия и основные принципы

Обработка ошибок в Lang строится на шести принципах. Любое изменение спецификации должно им соответствовать.

**П1. Ошибка — это значение.**
Ожидаемые сбои возвращаются как данные (`Result<T, E>`), а не выбрасываются как исключения. Поток управления не прерывается неявно.

**П2. Ошибки типизированы.**
Тип ошибки всегда известен компилятору. `Result<T, E>` с конкретным `E`. Нетипизированный «мешок» (`Dynamic`) допускается только на границах приложения (см. §12).

**П3. Необработанная ошибка — ошибка компиляции.**
Если функция возвращает `Result`, его нельзя проигнорировать. Компилятор требует либо `?`, либо явной обработки, либо явного `discard` с обоснованием.

**П4. Композиция без бойлерплейта.**
Наборы ошибок объединяются структурно. Не требуется вручную писать конверсии между типами ошибок (в отличие от `From`-имплементаций в Rust).

**П5. Эргономика важнее пуризма.**
Частые случаи должны быть короткими. Если обработка ошибок неудобна, разработчики найдут обход (`unwrap`, пустой `catch`).

**П6. Паника — не ошибка.**
Баги (`null`, выход за границы, `unreachable`) отделены от ожидаемых сбоев на уровне типов и рантайма.

---

## 2. Терминология

| Термин | Определение |
|---|---|
| **Ошибка (error)** | Ожидаемый сбой операции. Представлен значением типа, реализующего `Error`. |
| **Результат (Result)** | Значение типа `Result<T, E>`: либо успех (`Ok(T)`), либо ошибка (`Err(E)`). |
| **Набор ошибок (error set)** | Структурный тип-объединение видов ошибок, которые функция может вернуть. |
| **Паника (panic)** | Аварийное завершение из-за бага. Не является частью обычного потока ошибок. |
| **Граница (boundary)** | Точка, где типизированный код встречается с нетипизированным (JS interop, `main`, HTTP-обработчик). |
| **Контекст ошибки** | Запись о месте вызова (`file:line:col`), добавляемая при каждом `?`. |
| **`Dynamic`** | Специальный тип ошибки для границ, способный хранить любую ошибку + цепочку контекста. |

---

## 3. Типы ошибок

### 3.1. Требование к типу ошибки

Тип может использоваться в позиции ошибки (`E` в `Result<T, E>`, в наборе `throws {…}`), если он реализует встроенный интерфейс `Error`:

```lang
interface Error {
    fn message(self) -> String
}
```

Обычно это `enum` (алгебраический тип данных):

```lang
enum IoError implements Error {
    NotFound,
    PermissionDenied,
    Other(code: Int),
}

impl Error for IoError {
    fn message(self) -> String {
        match self {
            IoError.NotFound => "file not found",
            IoError.PermissionDenied => "permission denied",
            IoError.Other(code) => "io error {code}",
        }
    }
}
```

### 3.2. Правила

- Тип ошибки **должен** быть `enum` или `struct`, реализующим `Error`. Замыкания, примитивы и `null` — запрещены.
- Рекомендуется использовать `enum` для исчерпывающего матчинга.
- Ошибки **не наследуются** (нет иерархий классов). Композиция — только через наборы ошибок (§6).

### 3.3. Встроенные ошибки

Стандартная библиотека предоставляет:

- `IoError` — файлы, сеть
- `ParseError` — парсинг чисел, JSON, форматов
- `JsError` — обёртка над исключениями JavaScript (см. §13)
- `PanicError` — только для `catch_panic` (§11)

---

## 4. Тип `Result<T, E>`

### 4.1. Определение

```lang
enum Result<T, E: Error> {
    Ok(value: T),
    Err(error: E),
}
```

`Result` — обычный enum, к нему применимы все операции над enum. Псевдоним `Result<T>` = `Result<T, Dynamic>` (§12) допускается только в позиции возврата `main` и явно объявленных границ.

### 4.2. Методы

```lang
impl<T, E: Error> Result<T, E> {
    // Распаковка
    fn is_ok(self) -> Bool
    fn is_err(self) -> Bool
    fn unwrap(self) -> T                          // panic при Err
    fn unwrap_or(self, default: T) -> T
    fn unwrap_or_else(self, f: fn(E) -> T) -> T
    fn expect(self, msg: String) -> T             // panic с сообщением

    // Преобразования
    fn map<U>(self, f: fn(T) -> U) -> Result<U, E>
    fn map_err<F: Error>(self, f: fn(E) -> F) -> Result<T, F>
    fn and_then<U, F: Error>(self, f: fn(T) -> Result<U, F>) -> Result<U, E | F>
    fn or_else<F: Error>(self, f: fn(E) -> Result<T, F>) -> Result<T, F>
    fn flatten(self) -> Result<T, E>              // для Result<Result<T,E>,E>

    // Свёртка
    fn fold<U>(self, on_ok: fn(T) -> U, on_err: fn(E) -> U) -> U
}
```

**Семантика:**
- `unwrap()` и `expect()` вызывают `panic!` при `Err`. В обычном коде использовать **запрещено** (компилятор выдаёт warning `W1001`); допустимо только в тестах и при доказанной невозможности `Err`.
- `and_then` объединяет наборы ошибок автоматически: `E | F` — это структурный набор (§6).

---

## 5. Сигнатуры функций: `throws`

### 5.1. Синтаксис

```lang
fn read_file(path: String) -> String throws IoError { ... }
```

Это **синтаксический сахар** для:

```lang
fn read_file(path: String) -> Result<String, IoError> { ... }
```

Форма `-> Result<T, E>` и `-> T throws E` полностью эквивалентны. Рекомендуется использовать `throws` для читаемости.

### 5.2. Правила

- Функция **без** `throws` и **без** `Result` в возвращаемом типе не может вернуть ошибку. Компилятор проверяет это статически.
- `throws` допускается только на функциях, методах и замыканиях с явной сигнатурой.
- `throws` в замыканиях выводится автоматически, если замыкание содержит `?` (для inline-замыканий в `map`, `and_then` и т. п.):
  ```lang
  let names = users.map(|u| validate(u)?)   // замыкание: throws ValidationError
  ```

### 5.3. Наследование в трейтах

Метод трейта может объявлять `throws`, но реализация не может бросать **дополнительные** ошибки (только подмножество объявленного набора):

```lang
trait Storage {
    fn load(self, key: String) -> Bytes throws IoError
}

impl Storage for S3Storage {
    fn load(self, key: String) -> Bytes throws IoError { ... } // только IoError
}
```

---

## 6. Наборы ошибок (error sets)

### 6.1. Определение

**Набор ошибок** — это структурный тип, обозначающий «функция может вернуть одну из перечисленных ошибок». Синтаксис:

```lang
fn load(path: String) -> Config throws {IoError, ParseError}
```

### 6.2. Правила объединения

**Правило 6.2.1 (расширение).** Если функция `f` объявляет набор `S1`, а вызывается в функции `g`, объявляющей `S2`, то `S1 ⊆ S2` (теоретико-множественное включение). Никаких ручных конверсий не требуется.

```lang
fn read(p: String) -> String throws IoError { ... }
fn parse(s: String) -> Config throws ParseError { ... }

fn load(p: String) -> Config throws {IoError, ParseError} {
    let text = read(p)?       // IoError ⊆ {IoError, ParseError} — OK
    parse(text)?              // ParseError ⊆ {IoError, ParseError} — OK
}
```

**Правило 6.2.2 (подтипирование).** Функция с меньшим набором может использоваться везде, где ожидается больший набор:

```lang
let f: fn(String) -> Config throws {IoError, ParseError} = load
//                                  ^^^^^^^^^^^^^^^^^^^^^
// функции с набором {IoError} тоже подойдут
```

Это решает проблему checked exceptions в Java: расширение набора в вызываемой функции **не требует** изменения сигнатур всех вызывающих, если они сами не хотят пробрасывать новую ошибку.

**Правило 6.2.3 (пустой набор).** `throws {}` эквивалентно отсутствию `throws`. Функция не возвращает ошибок.

**Правило 6.2.4 (дубликаты).** Набор не содержит дубликатов и упорядочен канонически (по имени типа) при выводе типов и в сообщениях компилятора.

### 6.3. Вывод наборов

В **приватных** функциях (не публичных модуля API) набор можно опустить:

```lang
fn helper(x: Int) -> String throws {   // компилятор выведет набор
    // ...
}
```

В **публичных** сигнатурах набор **обязателен** — это контракт API.

### 6.4. Сужение набора

Явное сужение (например, после обработки части ошибок):

```lang
fn narrow(result: Result<T, {A, B}>) -> Result<T, B> throws A {
    match result {
        Ok(v) => Ok(v),
        Err(A) => raise A,
        Err(B) => Err(B),
    }
}
```

---

## 7. Оператор `?`

### 7.1. Семантика

`expr?` — если `expr` имеет тип `Result<T, E>`:

- при `Ok(v)` — выражение равно `v` типа `T`;
- при `Err(e)` — **немедленный возврат** `Err(e)` (с добавлением контекста, §9) из текущей функции.

### 7.2. Требования

- Текущая функция **должна** объявлять `throws`, и `E ⊆ throws-set`.
- `?` запрещён в функции без `throws`.
- `?` применим к `Result<T, E>` и к `Option<T>` (см. §7.5).

### 7.3. Порядок вычислений

В выражении `foo(a()?, b()?)`:

1. Вычисляется `a()`. Если `Err` — немедленный возврат, `b()` **не вызывается**.
2. Иначе вычисляется `b()`. Если `Err` — возврат.
3. Иначе вызывается `foo`.

Порядок слева направо; side effects сохраняются.

### 7.4. `?` в замыканиях

`?` внутри замыкания возвращает из **замыкания**, а не из внешней функции:

```lang
let results: Array<Result<Int, E>> = items.map(|x| parse(x)?)
//                                    ^^^^^^^^^^^^^^^^^^^^^^^^
// каждый элемент — Result, ? возвращает из замыкания
```

### 7.5. `?` для `Option<T>`

`?` работает и для `Option`:

```lang
fn first_word_len(s: String) -> Option<Int> {
    let w = s.split_whitespace().next()?    // None — ранний возврат
    Some(w.len())
}
```

В функции, возвращающей `Result`, `?` для `Option` **запрещён** — требуется явное `ok_or_else`:

```lang
let w = s.split_whitespace().next()
         .ok_or_else(|| ParseError.Empty)?
```

### 7.6. Запрещённые конструкции

- `?` в аргументах `panic!`, `assert!` — запрещён.
- `?` в `const`/`static` инициализаторах — запрещён.
- `?` в `main` без `throws Dynamic` — запрещён.

---

## 8. Обработка ошибок: `match`, `catch`, `recover`

### 8.1. `match` по `Result`

Основной способ — исчерпывающий `match`:

```lang
match load("config.toml") {
    Ok(cfg) => run(cfg),
    Err(IoError.NotFound) => log("no config"),
    Err(IoError.PermissionDenied) => log("no access"),
    Err(IoError.Other(code)) => log("io error {code}"),
    Err(ParseError.InvalidChar(c)) => log("bad char {c}"),
    Err(ParseError.OutOfRange) => log("out of range"),
}
```

Компилятор проверяет **исчерпывающость** по полному набору ошибок функции `load`. Пропустить случай нельзя.

### 8.2. `catch` — сокращённая форма

Синтаксический сахар для `match`, обрабатывающий ошибки одного типа:

```lang
let cfg = load("config.toml") catch |e: IoError| {
    log("io error: {e.message()}")
    return default_config()
}
```

Эквивалент:

```lang
let cfg = match load("config.toml") {
    Ok(v) => v,
    Err(e: IoError) => {
        log("io error: {e.message()}")
        return default_config()
    }
    Err(e) => return Err(e),      // прочие ошибки пробрасываются
}
```

`catch` **не** ловит `panic!` — только `Err` из `Result`.

### 8.3. `recover` — превращение ошибки в успех

```lang
let cfg = load("config.toml").recover(|e| default_config())
```

Или с типизированной обработкой:

```lang
let cfg = load("config.toml").recover(|e: IoError| {
    match e {
        IoError.NotFound => default_config(),
        _ => raise e,                     // повторный проброс
    }
})
```

### 8.4. `raise` — явный проброс

`raise e` — явный возврат `Err(e)` из текущей функции. Эквивалент `return Err(e)` для `throws`-функций. Обязателен, если `e` не входит в набор автоматически.

```lang
fn f() -> Int throws {A, B} {
    let x = g()?;                  // A | B из g()
    if x < 0 { raise A.Negative }
    Ok(x)
}
```

---

## 9. Контекст ошибок и стек-трейсы

### 9.1. Накопление контекста

Каждый `?` добавляет точку вызова в цепочку контекста ошибки. Стоимость — **ноль на успешном пути**: запись производится только при `Err`.

```lang
fn read_config(p: String) -> String throws IoError { ... }

fn load() -> Config throws {IoError, ParseError} {
    let text = read_config("config.toml")?    // контекст: load, line 3
    parse(text)?                              // контекст: load, line 4
}
```

Если `read_config` вернёт `IoError.NotFound`, итоговая цепочка:

```
IoError.NotFound
  at load (src/main.lang:3:16)
  at main (src/main.lang:10:5)
```

### 9.2. Ручное добавление контекста

```lang
load("config.toml").context("loading configuration")?
```

Строка контекста добавляется к цепочке и попадает в `message()` при выводе.

### 9.3. Формат вывода

`Error::message()` для ошибки с контекстом возвращает:

```
<сообщение ошибки>
  at <file>:<line>:<col>
  at <file>:<line>:<col>
  ...
```

### 9.4. Настоящие стек-трейсы

Для диагностики в dev-режиме:

```lang
load("config.toml").with_stacktrace()?
```

Это **дорогая** операция (`new Error().stack` в V8). Разрешено в dev-сборках и на границах (§12). В production-коде — запрещено линтером (`W1002`).

### 9.5. Взаимодействие с source maps

Транспайлер обязан генерировать source maps. Отладочные инструменты сопоставляют `file:line:col` из контекста с исходным `.lang`-файлом.

---

## 10. Асинхронность

### 10.1. `async fn` и `throws`

`async` и `throws` ортогональны:

```lang
async fn fetch(url: Url) -> Body throws {Timeout, HttpError} {
    let resp = http_get(url).await?
    resp.body().await?
}
```

### 10.2. Инвариант «никогда не reject»

**Наши асинхронные функции всегда возвращают resolved `Promise` с `Result` и никогда не reject-ятся.** Исключения JS (`throw`) внутри наших `async fn` не происходят — единственное исключение — `panic!` (§11), который на границе ловится в `catch_panic`.

Этот инвариант — краеугольный камень всего дизайна. Он проверяется:

- статически — компилятор не генерирует `throw` в теле функции;
- тестами — property-based тест «для любого входа промис резолвится»;
- рантайм-ассертом в dev-режиме — `process.on('unhandledRejection')` в тестах падает.

### 10.3. Параллельное выполнение

```lang
// Все результаты, включая ошибки
let results: Array<Result<Body, E>> = join_all(urls.map(fetch)).await

// Первая ошибка отменяет остальные
let bodies: Array<Body> = try_join_all(urls.map(fetch)).await?
```

`try_join_all`:

- ждёт все промисы;
- при первой `Err` отменяет остальные задачи (через `AbortController`);
- возвращает `Err` с контекстом «первая ошибка из N задач».

### 10.4. Таймауты

```lang
let body = fetch(url).timeout(3.seconds).await?
// возвращает Err(TimeoutError) если не успели
```

### 10.5. Отмена

Отмена корутины/задачи — **не ошибка**. Она не появляется в `Result`. Отменённая задача возвращает `Cancelled`-маркер, который проверяется на уровне `join`:

```lang
let task = spawn(fetch(url))
// ...
task.cancel()
match task.await {
    Completed(result) => handle(result),
    Cancelled => log("cancelled"),
}
```

---

## 11. Паники (`panic!`)

### 11.1. Назначение

`panic!` — **только** для багов, которые не должны происходить в корректной программе:

- `null`/`None` в месте, где значение обязательно;
- нарушение инварианта;
- `unreachable` в exhaustive-match;
- провал `assert!`.

### 11.2. Синтаксис

```lang
panic!("unreachable: state = {state}")
assert!(x > 0, "x must be positive")
```

### 11.3. Тип функции

Функция, которая может паниковать (явно или через вызовы), должна объявлять `abort`:

```lang
fn divide(a: Int, b: Int) -> Int abort {
    if b == 0 { panic!("division by zero") }
    a / b
}
```

`abort` **не** является частью `Result` и **не** появляется в наборах ошибок. Компилятор проверяет: если функция вызывает `panic!` или функцию с `abort`, она должна объявлять `abort`.

### 11.4. `catch_panic`

Единственный способ поймать панику — границы:

```lang
fn http_handler(req: Request) -> Response {
    catch_panic(|| {
        handle(req)
    }) match {
        Ok(resp) => resp,
        Panicked(p) => {
            log("panic: {p.message()}")
            Response.internal_server_error()
        }
    }
}
```

`catch_panic`:

- ловит `PanicError` (JS-исключение, §11.5);
- **не** ловит отмену (см. §10.5);
- **не** ловит `Result`-ошибки;
- допускается **только** на объявленных границах (HTTP-обработчик, `main`, FFI, тест).

### 11.5. Компиляция в JS

`panic!` компилируется в `throw new PanicError(message, context)`. `catch_panic` — в `try { … } catch (e) { if (e instanceof PanicError) … else throw e }`.

---

## 12. Тип `Dynamic` и границы приложения

### 12.1. Определение

`Dynamic` — специальный тип ошибки, способный хранить **любую** ошибку плюс цепочку контекста:

```lang
struct Dynamic implements Error {
    fn message(self) -> String
}
```

### 12.2. Использование

`Dynamic` допускается **только** в явно объявленных позициях:

- тип возврата `main`;
- тип ошибки в объявлении границы (`@boundary`-атрибут);
- `Result<T>` (короткая форма) допустим только в этих позициях.

Во всех остальных местах `Dynamic` **запрещён** линтером (`E2001`). Это защищает от «размывания» типов, как в `anyhow` в Rust.

### 12.3. Пример границы

```lang
@boundary
fn cli_entry(args: Array<String>) -> ExitCode throws Dynamic {
    let cfg = load_config("config.toml")?    // IoError | ParseError → Dynamic
    let result = run(cfg)?                   // любой тип ошибки → Dynamic
    ExitCode.Success
}
```

### 12.4. Извлечение типизированной ошибки

На границе `Dynamic` можно попытаться извлечь исходную ошибку:

```lang
match err.downcast<IoError>() {
    Some(io) => log("io error: {io.message()}"),
    None => log("unknown: {err.message()}"),
}
```

---

## 13. Интероп с JavaScript

### 13.1. Основной инвариант

**Наш код не бросает исключений JS, кроме `PanicError`.** JS-исключения могут приходить только от внешних функций, вызываемых из нашего кода, и оборачиваются в `JsError` на границе вызова.

### 13.2. Вызов JS-функции

```lang
extern fn json_parse(s: String) -> Json throws JsError

let v = json_parse(text)?
```

Компилируется в:

```js
let _t;
try { _t = {ok: true, value: JSON.parse(text)}; }
catch (e) { _t = {ok: false, error: JsError.fromException(e)}; }
if (!_t.ok) return withContext(_t, "src/main.lang", 5, 16);
const v = _t.value;
```

### 13.3. Вызов JS-Promise

```lang
extern fn fetch_js(url: String) -> Response throws JsError   // возвращает Promise

let resp = fetch_js(url).await?
```

Компилируется с `try/catch` вокруг `await`:

```js
let _t;
try { _t = {ok: true, value: await fetch(url)}; }
catch (e) { _t = {ok: false, error: JsError.fromException(e)}; }
```

### 13.4. Экспорт наших функций в JS

Наши `Result`-функции экспортируются как объекты `{ok, value|error}`. Для удобства JS-потребителей можно явно объявить адаптер:

```lang
@js_export(throwing)
fn load_config(path: String) -> Config throws IoError { ... }
```

Генерирует JS-обёртку:

```js
export function load_config(path) {
    const r = load_config_impl(path);
    if (r.ok) return r.value;
    throw r.error;
}
```

### 13.5. Обёртка JS-функции, которая бросает

```lang
@js_import(from_throwing)
extern fn legacy_parse(s: String) -> Json throws JsError
```

Автоматически оборачивается в try/catch.

### 13.6. `JsError`

```lang
struct JsError implements Error {
    cause: any,              // исходное JS-исключение
    fn message(self) -> String  // cause.message || String(cause)
}
```

`JsError` не сериализуется в JSON автоматически — сохраняйте `cause` для отладки.

---

## 14. Компиляция в JavaScript: правила и инварианты

### 14.1. Представление `Result`

```js
{ok: true,  value: v}
{ok: false, error: e}
```

Для TS-потребителей генерируется:

```ts
type Result<T, E> = {ok: true; value: T} | {ok: false; error: E};
```

### 14.2. `?`

```lang
fn f() -> T throws E {
    let a = g()?;
    let b = h(a)?;
    b
}
```

Компилируется в:

```js
function f() {
    const _a = g();
    if (!_a.ok) return withContext(_a, __FILE, 3, 16);
    const a = _a.value;
    const _b = h(a);
    if (!_b.ok) return withContext(_b, __FILE, 4, 16);
    const b = _b.value;
    return {ok: true, value: b};
}
```

### 14.3. `?` в async

Тот же early return, `await` перед проверкой:

```js
async function f() {
    const _a = await g();
    if (!_a.ok) return withContext(_a, __FILE, 3, 16);
    // …
}
```

Наши `async fn` **никогда** не reject-ятся (кроме `panic!`).

### 14.4. Наборы ошибок

Наборы ошибок **исчезают** при компиляции. Никаких рантайм-тегов, массивов или проверок. Это чисто типовая конструкция.

### 14.5. `match` по `Result`

```lang
match r {
    Ok(v) => f(v),
    Err(IoError.NotFound) => g(),
    Err(e) => h(e),
}
```

```js
if (r.ok) { f(r.value); }
else switch (tagOf(r.error)) {
    case "IoError.NotFound": g(); break;
    default: h(r.error); break;
}
```

### 14.6. `withContext`

```js
function withContext(result, file, line, col) {
    if (result.ok) return result;
    result.error.context = result.error.context || [];
    result.error.context.push({file, line, col});
    return result;
}
```

Для успешного пути — один `if`. Для ошибки — push в массив.

### 14.7. Паника

```js
class PanicError extends Error { /* … */ }
function panic(msg, file, line, col) {
    throw new PanicError(msg, {file, line, col});
}
```

---

## 15. Стандартная библиотека: модуль `core.result`

```lang
module core.result

pub enum Result<T, E: Error> {
    Ok(value: T),
    Err(error: E),
}

pub fn ok<T, E: Error>(v: T) -> Result<T, E>
pub fn err<T, E: Error>(e: E) -> Result<T, E>

impl<T, E: Error> Result<T, E> {
    pub fn is_ok(self) -> Bool
    pub fn is_err(self) -> Bool
    pub fn unwrap(self) -> T
    pub fn unwrap_or(self, default: T) -> T
    pub fn unwrap_or_else(self, f: fn(E) -> T) -> T
    pub fn expect(self, msg: String) -> T
    pub fn map<U>(self, f: fn(T) -> U) -> Result<U, E>
    pub fn map_err<F: Error>(self, f: fn(E) -> F) -> Result<T, F>
    pub fn and_then<U, F: Error>(self, f: fn(T) -> Result<U, F>) -> Result<U, E | F>
    pub fn or_else<F: Error>(self, f: fn(E) -> Result<T, F>) -> Result<T, F>
    pub fn flatten(self) -> Result<T, E>
    pub fn fold<U>(self, on_ok: fn(T) -> U, on_err: fn(E) -> U) -> U
    pub fn context(self, msg: String) -> Result<T, E>
    pub fn with_stacktrace(self) -> Result<T, E>
}
```

Модули `core.io`, `core.parse`, `core.js` определяют соответствующие типы ошибок.

---

## 16. Стиль и рекомендации

### 16.1. Выбор формы сигнатуры

- Один тип ошибки → `throws E`.
- Несколько типов, но их немного (< 5) → `throws {A, B, C}`.
- Граница приложения → `throws Dynamic`.
- Публичный API с эволюционирующим набором ошибок → `Result<T, E>` с явным `E`-enum, расширяемым через `#[non_exhaustive]`.

### 16.2. Проектирование типов ошибок

- Используйте `enum`, а не `struct` — для исчерпывающего матчинга.
- Давайте осмысленные имена вариантам (`NotFound`, а не `Err1`).
- Не кладите в ошибку «сырые» детали реализации (стек-трейсы, connection strings) — только то, что полезно вызывающей стороне.
- Для библиотек — добавляйте `#[non_exhaustive]` к публичным `enum`, чтобы расширение не было ломающим изменением.

### 16.3. Когда `raise`, когда `?`

- `?` — когда ошибка «проходит насквозь».
- `raise` — когда нужно вернуть ошибку, не являющуюся результатом другого `Result`:
  ```lang
  if x < 0 { raise ValidationError.Negative }
  ```

### 16.4. Контекст

Добавляйте `.context("...")` на каждом уровне, где операция имеет смысл:

```lang
fn load_user(id: Int) -> User throws {DbError, NotFound} {
    db.query("...").context("loading user {id}").await?
}
```

Не добавляйте контекст в каждую функцию — только на значимых границах (публичный API, работа с внешними системами).

### 16.5. Асинхронность

- Всегда используйте `.timeout(...)` при работе с внешними системами.
- `try_join_all` — по умолчанию для параллельных задач; `join_all` — когда нужны все результаты.
- Не запускайте «висящие» задачи без `cancel`-стратегии.

### 16.6. Тесты

- `unwrap()` разрешён в тестах.
- `assert!` с осмысленным сообщением.
- Property-based тесты для инварианта «никогда не reject» (§10.2).

---

## 17. Антипаттерны

| Антипаттерн | Почему плохо | Правильно |
|---|---|---|
| `let _ = f()?` (игнорирование) | Компилятор требует явного `discard _ = f()?` с комментарием | `discard _ = f()? // ok: ошибка не критична` |
| `unwrap()` в продакшене | Паника при `Err` | `?` или `.unwrap_or_else(...)` |
| `Dynamic` в обычном коде | Теряет типизацию, как `throws Exception` | Типизированный `enum` + `throws` |
| `catch {}` пустой | Скрывает ошибки | Явный `match` или `.recover(...)` |
| `.with_stacktrace()` везде | Дорого | Только на границах в dev |
| `try_join_all` без таймаута | Может висеть бесконечно | `.timeout(...)` |
| `async fn` с `try/catch` внутри | Ломает инвариант «не reject» | Оставить try/catch только на границах JS |
| Ошибки как строки | Нет типизации | `enum` + `impl Error` |

---

## 18. Приложения

### 18.1. Полный пример

```lang
import core.io.{read_file, IoError}
import core.parse.{parse_int, ParseError}
import core.result.Result

enum ConfigError implements Error {
    InvalidPort(port: Int),
    fn message(self) -> String {
        match self {
            ConfigError.InvalidPort(p) => "invalid port: {p}"
        }
    }
}

struct Config { port: Int, workers: Int }

fn load(path: String) -> Config throws {IoError, ParseError, ConfigError} {
    let text = read_file(path).context("reading config")?
    let port = parse_int(find_field(text, "port")).context("parsing port")?
    if port < 1 || port > 65535 {
        raise ConfigError.InvalidPort(port)
    }
    let workers = parse_int(find_field(text, "workers"))?
    Config { port, workers }
}

@boundary
fn main(args: Array<String>) -> ExitCode throws Dynamic {
    match load("config.toml") {
        Ok(cfg) => run(cfg),
        Err(e) => {
            eprintln!("{e.message()}")
            ExitCode.Failure
        }
    }
}

fn run(cfg: Config) -> ExitCode abort {
    // ...
    ExitCode.Success
}
```

### 18.2. Компиляция `load` в JavaScript

```js
function load(path) {
    const _text = read_file(path);
    if (!_text.ok) return withContext(_text, "src/config.lang", 14, 16);
    const text = _text.value;

    const _port = parse_int(find_field(text, "port"));
    if (!_port.ok) return withContext(_port, "src/config.lang", 15, 16);
    const port = _port.value;

    if (port < 1 || port > 65535) {
        return {ok: false, error: {tag: "ConfigError.InvalidPort", port}};
    }

    const _workers = parse_int(find_field(text, "workers"));
    if (!_workers.ok) return withContext(_workers, "src/config.lang", 19, 20);
    const workers = _workers.value;

    return {ok: true, value: {port, workers}};
}
```

### 18.3. Глоссарий

- **Boundary** — граница приложения, где допускается `Dynamic`.
- **Context** — цепочка `file:line:col`, накапливаемая при `?`.
- **Discard** — явное игнорирование `Result` с комментарием.
- **Error set** — структурный тип «набор ошибок» функции.
- **Panic** — аварийное завершение из-за бага.
- **Reject** — отклонённый Promise в JS; в Lang не порождается нашим кодом.
- **Unwrap** — извлечение значения из `Result` с паникой при `Err`.

### 18.4. Таблица кодов ошибок компилятора

| Код | Тип | Описание |
|---|---|---|
| `E1001` | error | `?` вне `throws`-функции |
| `E1002` | error | `unwrap()` в продакшене |
| `E1003` | error | `Dynamic` в не-границе |
| `E1004` | error | Пропущен случай в `match` по набору |
| `E1005` | error | `?` для `Option` в `Result`-функции |
| `E2001` | error | Набор ошибок содержит не-`Error` тип |
| `E2002` | error | `abort`-функция вызывается без `abort` в сигнатуре |
| `W1001` | warn | `expect()`/`unwrap()` вне теста |
| `W1002` | warn | `with_stacktrace()` в production |

---

**Конец документа.** Вопросы и предложения — в репозиторий спецификации.